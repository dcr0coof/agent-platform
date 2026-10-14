package trip

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAmbiguousRequestsDoNotMutateSessions(t *testing.T) {
	h := setup(t, DemoRunner{Delay: time.Millisecond})
	s := h.newSession()
	for _, tc := range []struct{ method, path, body string }{
		{"POST", "/api/sessions", `{"title":"one","title":"two"}`},
		{"POST", "/api/sessions", `{"constraints":{"budget":10,"budg\u0065t":999}}`},
		{"POST", "/api/sessions", `{"constraints":{},"CONSTRAINTS":{"budget":999}}`},
		{"POST", "/api/sessions", `{"constraints":{},"conſtraints":{}}`},
		{"POST", "/api/sessions", `null`},
		{"PUT", "/api/sessions/" + s.ID, fmt.Sprintf(`{"revision":%d,"Revision":%d,"title":"changed"}`, s.Revision, s.Revision)},
		{"PUT", "/api/sessions/" + s.ID, fmt.Sprintf(`{"revision":%d,"constraints":{"budget":10,"BUDGET":20}}`, s.Revision)},
		{"POST", "/api/sessions/" + s.ID + "/runs", fmt.Sprintf(`{"revision":%d,"message":"one","message":"two","request_id":"first-key"}`, s.Revision)},
		{"POST", "/api/sessions/" + s.ID + "/runs", fmt.Sprintf(`{"revision":%d,"message":"one","request_id":"first-key","REQUEST_ID":"second-key"}`, s.Revision)},
	} {
		t.Run(tc.body, func(t *testing.T) {
			h.request(tc.method, tc.path, json.RawMessage(tc.body), 400, nil)
			var got Session
			h.request("GET", "/api/sessions/"+s.ID, nil, 200, &got)
			if got.Revision != s.Revision || got.Title != s.Title || got.Constraints != s.Constraints || got.ActiveRun != nil || len(got.Messages) != 0 {
				t.Fatalf("invalid request changed session: %+v", got)
			}
		})
	}
	var sessions []Session
	h.request("GET", "/api/sessions", nil, 200, &sessions)
	if len(sessions) != 1 {
		t.Fatal("ambiguous create persisted a session")
	}
	h.request("PUT", "/api/sessions/"+s.ID, sessionInput{Title: "valid", Revision: s.Revision, Constraints: Constraints{Budget: 20}}, 200, &s)
	if s.Title != "valid" || s.Constraints.Budget != 20 {
		t.Fatal("valid update did not recover")
	}
}

func TestRequestJSONExistingBoundaries(t *testing.T) {
	for _, body := range []string{`{"title":"a"} {}`, `{"unknown":1}`, `{"constraints":{"budget":"bad"}}`, `[]`, `{"title":"` + strings.Repeat("x", 32<<10) + `"}`} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		var input sessionInput
		if readJSON(w, r, &input) || w.Code != 400 {
			t.Fatal("invalid/oversized request accepted")
		}
	}
	// Equal keys in separate objects and duplicate-looking text are not duplicates.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"title":"root","child":{"title":"nested"},"text":"\"title\":1,\"title\":2"}`))
	r.Header.Set("Content-Type", "application/json")
	var input struct {
		Title, Text string
		Child       struct{ Title string }
	}
	if !readJSON(w, r, &input) || input.Child.Title != "nested" {
		t.Fatal("independent objects or text rejected")
	}
}

func TestJSONMediaTypes(t *testing.T) {
	for _, tc := range []struct {
		header string
		valid  bool
	}{
		{"application/json", true}, {"Application/JSON", true},
		{"application/json; charset=utf-8", true}, {`application/json; charset="UTF-8"`, true},
		{"", false}, {"application/jsonp", false}, {"application/json-extra", false},
		{"text/plain", false}, {"application/json; charset", false},
		{`application/json; charset="unfinished`, false},
		{"application/json, text/plain", false},
	} {
		t.Run(tc.header, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/", strings.NewReader(`{"title":"ok"}`))
			r.Header.Set("Content-Type", tc.header)
			var input sessionInput
			if ok := readJSON(w, r, &input); ok != tc.valid || (!ok && w.Code != 415) {
				t.Fatalf("accepted=%v status=%d", ok, w.Code)
			}
		})
	}
}

func TestRejectedMediaTypeDoesNotMutate(t *testing.T) {
	h := setup(t, DemoRunner{Delay: time.Millisecond})
	s := h.newSession()
	for _, tc := range []struct{ method, path, body string }{
		{"POST", "/api/sessions", `{"title":"unexpected"}`},
		{"PUT", "/api/sessions/" + s.ID, fmt.Sprintf(`{"revision":%d,"title":"unexpected"}`, s.Revision)},
		{"POST", "/api/sessions/" + s.ID + "/runs", encode(startBody("unexpected", "media-key", s.Revision))},
	} {
		req, _ := http.NewRequest(tc.method, h.server.URL+tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/jsonp")
		resp, err := h.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 415 {
			t.Fatalf("%s %s: %d", tc.method, tc.path, resp.StatusCode)
		}
	}
	var got Session
	h.request("GET", "/api/sessions/"+s.ID, nil, 200, &got)
	if got.Revision != s.Revision || got.Title != s.Title || got.LatestRun != nil || len(got.Messages) != 0 {
		t.Fatalf("rejected request changed state: %+v", got)
	}
	var list []Session
	h.request("GET", "/api/sessions", nil, 200, &list)
	if len(list) != 1 {
		t.Fatal("rejected create persisted a session")
	}
}
