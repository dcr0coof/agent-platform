package trip

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestLatestRunSurvivesReloadWithoutChangingHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trips.db")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	ss, err := s.Create("owner", "trip", Constraints{})
	if err != nil {
		t.Fatal(err)
	}
	for i, status := range []string{"failed", "cancelled", "completed"} {
		r, fresh, err := s.Start("owner", ss.ID, status, status, ss.Revision)
		if err != nil || !fresh {
			t.Fatalf("start: %v fresh=%v", err, fresh)
		}
		// Deliberately reverse timestamps. Latest is creation order, not lexical time.
		if _, err := s.db.Exec("UPDATE runs SET created_at=? WHERE id=?", []string{"2030-01-01", "2020-01-01", "2020-01-01"}[i], r.ID); err != nil {
			t.Fatal(err)
		}
		active, err := s.Get("owner", ss.ID)
		if err != nil || active.ActiveRun == nil || active.LatestRun == nil || active.ActiveRun.ID != r.ID || active.LatestRun.ID != r.ID {
			t.Fatalf("active: %+v %v", active, err)
		}
		if err := s.Finish(r.ID, status, "terminal detail", nil); err != nil {
			t.Fatal(err)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		s, err = OpenStore(path)
		if err != nil {
			t.Fatal(err)
		}
		ss, err = s.Get("owner", ss.ID)
		if err != nil || ss.ActiveRun != nil || ss.LatestRun == nil || ss.LatestRun.ID != r.ID || ss.LatestRun.Status != status || ss.LatestRun.Input != status || len(ss.Messages) != 0 {
			t.Fatalf("terminal reload: %+v %v", ss, err)
		}
		if events := ss.LatestRun.Events; len(events) != 2 || events[1].Type != "run."+status || events[1].Detail != "terminal detail" {
			t.Fatalf("events missing: %+v", events)
		}
	}
	if _, err := s.Get("other-owner", ss.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("owner boundary: %v", err)
	}
	other, err := s.Create("owner", "other trip", Constraints{})
	if err != nil {
		t.Fatal(err)
	}
	other, err = s.Get("owner", other.ID)
	if err != nil || other.LatestRun != nil || other.ActiveRun != nil {
		t.Fatalf("unrelated trip: %+v %v", other, err)
	}
	list, err := s.List("owner")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range list {
		if item.LatestRun != nil || item.ActiveRun != nil {
			t.Fatal("list loaded run payloads")
		}
	}
}
