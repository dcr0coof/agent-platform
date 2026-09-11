package trip

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dcr0coof/agent-platform/internal/llm"
	_ "modernc.org/sqlite"
)

// Store is a single-process SQLite store. All terminal transitions and history
// commits are atomic. A database file must have only one running server owner.
type Store struct{ db *sql.DB }

func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	_, err = db.Exec(`PRAGMA busy_timeout=5000; PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;
 CREATE TABLE IF NOT EXISTS sessions (
 id TEXT PRIMARY KEY,owner TEXT NOT NULL,title TEXT NOT NULL,constraints_json TEXT NOT NULL,
 messages_json TEXT NOT NULL,revision INTEGER NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
 CREATE INDEX IF NOT EXISTS sessions_owner ON sessions(owner,updated_at);
 CREATE TABLE IF NOT EXISTS runs (
 id TEXT PRIMARY KEY,session_id TEXT NOT NULL REFERENCES sessions(id),request_id TEXT NOT NULL,
 input TEXT NOT NULL,status TEXT NOT NULL,error TEXT NOT NULL DEFAULT '',created_at TEXT NOT NULL,
 UNIQUE(session_id,request_id));
 CREATE INDEX IF NOT EXISTS runs_session ON runs(session_id,status);
 CREATE TABLE IF NOT EXISTS events (
 run_id TEXT NOT NULL REFERENCES runs(id),seq INTEGER NOT NULL,type TEXT NOT NULL,
 detail TEXT NOT NULL,created_at TEXT NOT NULL,PRIMARY KEY(run_id,seq));`)
	if err != nil {
		db.Close()
		return nil, err
	}
	// A crashed process cannot resume unknown in-flight tool side effects. Keep a
	// failed run record, leave chat history intact, and require a new user run.
	rows, err := db.Query("SELECT id FROM runs WHERE status='running'")
	if err != nil {
		db.Close()
		return nil, err
	}
	var pending []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			break
		}
		pending = append(pending, id)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		db.Close()
		return nil, err
	}
	for _, id := range pending {
		if err = s.Finish(id, "failed", "服务重启，运行已中断；请确认后重新发送", nil); err != nil {
			db.Close()
			return nil, err
		}
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }

func encode(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

type scanner interface{ Scan(...interface{}) error }

func scanSession(row scanner) (Session, error) {
	var s Session
	var c, m string
	err := row.Scan(&s.ID, &s.Title, &c, &m, &s.Revision, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return s, ErrNotFound
	}
	if err != nil {
		return s, err
	}
	if err = json.Unmarshal([]byte(c), &s.Constraints); err != nil {
		return s, err
	}
	err = json.Unmarshal([]byte(m), &s.Messages)
	return s, err
}

const sessionColumns = "id,title,constraints_json,messages_json,revision,created_at,updated_at"

func (s *Store) Create(owner, title string, c Constraints) (Session, error) {
	now := timestamp()
	ss := Session{ID: newID(), Title: title, Constraints: c, Messages: []llm.Message{}, Revision: 1, CreatedAt: now, UpdatedAt: now}
	_, err := s.db.Exec("INSERT INTO sessions VALUES(?,?,?,?,?,?,?,?)", ss.ID, owner, title, encode(c), "[]", 1, now, now)
	return ss, err
}
func (s *Store) Get(owner, id string) (Session, error) {
	ss, err := scanSession(s.db.QueryRow("SELECT "+sessionColumns+" FROM sessions WHERE owner=? AND id=?", owner, id))
	if err != nil {
		return ss, err
	}
	var runID string
	err = s.db.QueryRow("SELECT id FROM runs WHERE session_id=? AND status='running'", id).Scan(&runID)
	if err == nil {
		r, e := s.Run(owner, runID)
		if e != nil {
			return ss, e
		}
		ss.ActiveRun = &r
	} else if !errors.Is(err, sql.ErrNoRows) {
		return ss, err
	}
	return ss, nil
}
func (s *Store) List(owner string) ([]Session, error) {
	// Navigation needs metadata only; do not load or decode full chat histories.
	rows, err := s.db.Query("SELECT id,title,constraints_json,'null',revision,created_at,updated_at FROM sessions WHERE owner=? ORDER BY updated_at DESC", owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Session{}
	for rows.Next() {
		ss, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, ss)
	}
	return result, rows.Err()
}
func (s *Store) Update(owner, id, title string, c Constraints, revision int) (Session, error) {
	res, err := s.db.Exec(`UPDATE sessions SET title=?,constraints_json=?,revision=revision+1,updated_at=?
 WHERE owner=? AND id=? AND revision=? AND NOT EXISTS(SELECT 1 FROM runs WHERE session_id=? AND status='running')`, title, encode(c), timestamp(), owner, id, revision, id)
	if err != nil {
		return Session{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return Session{}, ErrConflict
	}
	return s.Get(owner, id)
}
func (s *Store) Start(owner, sid, input, requestID string, revision int) (Run, bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Run{}, false, err
	}
	defer tx.Rollback()
	var current int
	err = tx.QueryRow("SELECT revision FROM sessions WHERE id=? AND owner=?", sid, owner).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return Run{}, false, ErrNotFound
	}
	if err != nil {
		return Run{}, false, err
	}
	var prior, inputBefore string
	err = tx.QueryRow("SELECT id,input FROM runs WHERE session_id=? AND request_id=?", sid, requestID).Scan(&prior, &inputBefore)
	if err == nil {
		if inputBefore != input {
			return Run{}, false, ErrConflict
		}
		tx.Rollback()
		r, e := s.Run(owner, prior)
		return r, false, e
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Run{}, false, err
	}
	var active int
	if err = tx.QueryRow("SELECT count(*) FROM runs WHERE session_id=? AND status='running'", sid).Scan(&active); err != nil {
		return Run{}, false, err
	}
	if current != revision || active > 0 {
		return Run{}, false, ErrConflict
	}
	r := Run{ID: newID(), SessionID: sid, RequestID: requestID, Input: input, Status: "running", CreatedAt: timestamp(), Events: []Event{}}
	_, err = tx.Exec("INSERT INTO runs(id,session_id,request_id,input,status,created_at) VALUES(?,?,?,?,?,?)", r.ID, sid, requestID, input, r.Status, r.CreatedAt)
	if err != nil {
		return r, false, err
	}
	_, err = tx.Exec("INSERT INTO events VALUES(?,?,?,?,?)", r.ID, 1, "run.started", "任务已接收", r.CreatedAt)
	if err != nil {
		return r, false, err
	}
	err = tx.Commit()
	return r, true, err
}
func (s *Store) Run(owner, id string) (Run, error) {
	var r Run
	err := s.db.QueryRow(`SELECT r.id,r.session_id,r.request_id,r.input,r.status,r.error,r.created_at FROM runs r
 JOIN sessions s ON s.id=r.session_id WHERE s.owner=? AND r.id=?`, owner, id).Scan(&r.ID, &r.SessionID, &r.RequestID, &r.Input, &r.Status, &r.Error, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	if err != nil {
		return r, err
	}
	r.Events, err = s.Events(id, 0)
	return r, err
}
func (s *Store) Events(id string, after int) ([]Event, error) {
	rows, err := s.db.Query("SELECT seq,type,detail,created_at FROM events WHERE run_id=? AND seq>? ORDER BY seq", id, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []Event{}
	for rows.Next() {
		var e Event
		if err = rows.Scan(&e.Seq, &e.Type, &e.Detail, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
func (s *Store) AppendEvent(id, kind, detail string) error {
	res, err := s.db.Exec(`INSERT INTO events SELECT ?,COALESCE((SELECT MAX(seq) FROM events WHERE run_id=?),0)+1,?,?,?
 WHERE EXISTS(SELECT 1 FROM runs WHERE id=? AND status='running')`, id, id, kind, detail, timestamp(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return context.Canceled
	}
	return nil
}
func (s *Store) Finish(id, status, detail string, added []llm.Message) error {
	if status != "completed" && status != "failed" && status != "cancelled" {
		return fmt.Errorf("invalid terminal status")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var sid, current string
	if err = tx.QueryRow("SELECT session_id,status FROM runs WHERE id=?", id).Scan(&sid, &current); err != nil {
		return err
	}
	if current != "running" {
		return nil
	}
	if status == "completed" {
		var raw string
		var history []llm.Message
		if err = tx.QueryRow("SELECT messages_json FROM sessions WHERE id=?", sid).Scan(&raw); err != nil {
			return err
		}
		if err = json.Unmarshal([]byte(raw), &history); err != nil {
			return err
		}
		history = append(history, added...)
		if _, err = tx.Exec("UPDATE sessions SET messages_json=?,revision=revision+1,updated_at=? WHERE id=?", encode(history), timestamp(), sid); err != nil {
			return err
		}
	}
	failure := ""
	if status != "completed" {
		failure = detail
	}
	if _, err = tx.Exec("UPDATE runs SET status=?,error=? WHERE id=?", status, failure, id); err != nil {
		return err
	}
	if _, err = tx.Exec("INSERT INTO events SELECT ?,COALESCE(MAX(seq),0)+1,?,?,? FROM events WHERE run_id=?", id, "run."+status, detail, timestamp(), id); err != nil {
		return err
	}
	return tx.Commit()
}
