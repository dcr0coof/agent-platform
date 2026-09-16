package trip

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type HTTP struct {
	Service       *Service
	Demo          bool
	WebDir        string
	AllowedOrigin string
}

func (h *HTTP) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/bootstrap", h.bootstrap)
	mux.HandleFunc("GET /api/sessions", h.sessions)
	mux.HandleFunc("POST /api/sessions", h.create)
	mux.HandleFunc("GET /api/sessions/{id}", h.session)
	mux.HandleFunc("PUT /api/sessions/{id}", h.update)
	mux.HandleFunc("POST /api/sessions/{id}/runs", h.start)
	mux.HandleFunc("GET /api/runs/{id}", h.run)
	mux.HandleFunc("POST /api/runs/{id}/cancel", h.cancel)
	mux.HandleFunc("GET /api/runs/{id}/events", h.events)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { problem(w, 404, "接口不存在") })
	mux.HandleFunc("/", h.static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
			// Browser clients may only mutate/read through the same origin. No CORS.
			if origin := r.Header.Get("Origin"); origin != "" && origin != h.AllowedOrigin {
				u, err := url.Parse(origin)
				scheme := "http"
				if r.TLS != nil {
					scheme = "https"
				}
				if err != nil || u.Host != r.Host || u.Scheme != scheme {
					problem(w, 403, "跨站请求被拒绝")
					return
				}
			}
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				problem(w, 403, "跨站请求被拒绝")
				return
			}
			host := r.Host
			if value, _, err := net.SplitHostPort(host); err == nil {
				host = value
			}
			if host != "localhost" && host != "127.0.0.1" && host != "::1" {
				problem(w, 403, "此版本仅用于本机工作台")
				return
			}
			if r.URL.Path != "/api/bootstrap" && owner(r) == "" {
				problem(w, 401, "会话标识已失效，请刷新页面")
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}

const cookieName = "trip_workspace"

func owner(r *http.Request) string {
	c, err := r.Cookie(cookieName)
	if err != nil || len(c.Value) != 64 {
		return ""
	}
	b, err := hex.DecodeString(c.Value)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func (h *HTTP) bootstrap(w http.ResponseWriter, r *http.Request) {
	if owner(r) == "" {
		var token [32]byte
		if _, err := rand.Read(token[:]); err != nil {
			problem(w, 500, "无法创建浏览器会话")
			return
		}
		http.SetCookie(w, &http.Cookie{Name: cookieName, Value: hex.EncodeToString(token[:]), Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: 365 * 24 * 3600})
	}
	respond(w, 200, map[string]interface{}{"demo": h.Demo, "name": "行迹", "storage": "sqlite", "capabilities": []string{"trip_constraints", "conversation", "run_events", "cancellation"}})
}
func respond(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, status int, message string) {
	respond(w, status, map[string]string{"error": message})
}
func fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		problem(w, 404, err.Error())
	case errors.Is(err, ErrConflict):
		problem(w, 409, err.Error())
	case errors.Is(err, ErrCapacity):
		problem(w, 429, err.Error())
	default:
		problem(w, 500, "存储或运行出错，请稍后重试")
	}
}
func readJSON(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		problem(w, 415, "请求必须使用 JSON")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		problem(w, 400, "请求格式错误或超过 32 KiB")
		return false
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		problem(w, 400, "只允许一个 JSON 对象")
		return false
	}
	return true
}

type sessionInput struct {
	Title       string      `json:"title"`
	Constraints Constraints `json:"constraints"`
	Revision    int         `json:"revision"`
}

func validateSession(input *sessionInput) error {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		input.Title = "新的出行"
	}
	if len([]rune(input.Title)) > 80 {
		return fmt.Errorf("标题最多 80 个字符")
	}
	return input.Constraints.Validate()
}
func (h *HTTP) sessions(w http.ResponseWriter, r *http.Request) {
	ss, err := h.Service.Store.List(owner(r))
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, ss)
}
func (h *HTTP) create(w http.ResponseWriter, r *http.Request) {
	var input sessionInput
	if !readJSON(w, r, &input) {
		return
	}
	if err := validateSession(&input); err != nil {
		problem(w, 400, err.Error())
		return
	}
	ss, err := h.Service.Store.Create(owner(r), input.Title, input.Constraints)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 201, ss)
}
func (h *HTTP) session(w http.ResponseWriter, r *http.Request) {
	ss, err := h.Service.Store.Get(owner(r), r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, ss)
}
func (h *HTTP) update(w http.ResponseWriter, r *http.Request) {
	var input sessionInput
	if !readJSON(w, r, &input) {
		return
	}
	if err := validateSession(&input); err != nil {
		problem(w, 400, err.Error())
		return
	}
	if _, err := h.Service.Store.Get(owner(r), r.PathValue("id")); err != nil {
		fail(w, err)
		return
	}
	ss, err := h.Service.Update(owner(r), r.PathValue("id"), input.Title, input.Constraints, input.Revision)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, ss)
}

var requestPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,80}$`)

func (h *HTTP) start(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
		Revision  int    `json:"revision"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	input.Message = strings.TrimSpace(input.Message)
	if input.Message == "" || len(input.Message) > 16000 || !requestPattern.MatchString(input.RequestID) {
		problem(w, 400, "消息不能为空且最多 16000 字节，请提供有效 request_id")
		return
	}
	run, err := h.Service.Start(owner(r), r.PathValue("id"), input.Message, input.RequestID, input.Revision)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 202, run)
}
func (h *HTTP) run(w http.ResponseWriter, r *http.Request) {
	run, err := h.Service.Store.Run(owner(r), r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, run)
}
func (h *HTTP) cancel(w http.ResponseWriter, r *http.Request) {
	run, err := h.Service.Cancel(owner(r), r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, run)
}
func (h *HTTP) events(w http.ResponseWriter, r *http.Request) {
	run, err := h.Service.Store.Run(owner(r), r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	last := r.Header.Get("Last-Event-ID")
	if last == "" {
		last = r.URL.Query().Get("after")
	}
	after := 0
	if last != "" {
		after, err = strconv.Atoi(last)
		if err != nil || after < 0 {
			problem(w, 400, "事件序号无效")
			return
		}
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(w)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	heartbeat := time.NewTicker(10 * time.Second)
	defer heartbeat.Stop()
	for {
		for _, event := range run.Events {
			if event.Seq > after {
				_ = controller.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if _, err = fmt.Fprintf(w, "id: %d\ndata: %s\n\n", event.Seq, encode(event)); err != nil {
					return
				}
				if err = controller.Flush(); err != nil {
					return
				}
				after = event.Seq
			}
		}
		if run.Terminal() {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			_ = controller.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if _, err = fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			if controller.Flush() != nil {
				return
			}
		case <-ticker.C:
		}
		run, err = h.Service.Store.Run(owner(r), run.ID)
		if err != nil {
			return
		}
	}
}
func (h *HTTP) static(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		w.WriteHeader(405)
		return
	}
	path := filepath.Join(h.WebDir, filepath.FromSlash(strings.TrimPrefix(r.URL.Path, "/")))
	root, err := filepath.Abs(h.WebDir)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	abs, err := filepath.Abs(path)
	if err != nil || !(abs == root || strings.HasPrefix(abs, root+string(os.PathSeparator))) {
		w.WriteHeader(404)
		return
	}
	if info, err := os.Stat(abs); err == nil && !info.IsDir() {
		http.ServeFile(w, r, abs)
		return
	}
	if strings.Contains(filepath.Base(r.URL.Path), ".") {
		http.NotFound(w, r)
		return
	}
	index := filepath.Join(root, "index.html")
	if _, err := os.Stat(index); err != nil {
		problem(w, 503, "前端尚未构建，请先在 web 目录运行 npm ci 和 npm run build")
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, index)
}
