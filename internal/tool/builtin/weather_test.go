package builtin

import (
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWeatherGzip(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		z := gzip.NewWriter(w)
		z.Write([]byte(`{"code":"200"}`))
		z.Close()
	}))
	defer s.Close()
	w := NewWeather("key", s.URL)
	data, err := w.doGet(context.Background(), s.URL)
	if err != nil || string(data) != `{"code":"200"}` {
		t.Fatalf("gzip not decoded: %q, %v", data, err)
	}
}

func TestWeatherLookupAndQueries(t *testing.T) {
	for _, kind := range []string{"now", "forecast"} {
		t.Run(kind, func(t *testing.T) {
			paths := []string{}
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				if r.Header.Get("X-QW-Api-Key") != "private-key" || r.URL.Query().Get("key") != "" {
					t.Error("key must only be in header")
				}
				switch r.URL.Path {
				case "/geo/v2/city/lookup":
					if r.URL.Query().Get("location") != "New York" {
						t.Error("bad city encoding")
					}
					w.Write([]byte(`{"code":"200","location":[{"id":"123","name":"New York"}]}`))
				case "/v7/weather/now":
					if r.URL.Query().Get("location") != "123" {
						t.Error("bad location ID")
					}
					w.Write([]byte(`{"code":"200","now":{"temp":"26","text":"晴"}}`))
				case "/v7/weather/3d":
					w.Write([]byte(`{"code":"200","daily":[{"fxDate":"2026-09-05","tempMax":"28","tempMin":"20","textDay":"晴"}]}`))
				default:
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
			}))
			defer s.Close()
			result, err := NewWeather("private-key", s.URL+"/").Execute(context.Background(), map[string]interface{}{"city": "New York", "type": kind})
			if err != nil || !strings.Contains(result, "晴") || len(paths) != 2 {
				t.Fatalf("weather failed: %s %v %v", result, err, paths)
			}
		})
	}
}

func TestWeatherRejectsBadInputBeforeNetwork(t *testing.T) {
	w := NewWeather("key", "http://127.0.0.1:1")
	for _, params := range []map[string]interface{}{
		nil, {"city": " "}, {"city": "北京", "type": "invalid"}, {"city": "北京", "type": 3},
	} {
		_, err := w.Execute(context.Background(), params)
		if err == nil || strings.Contains(err.Error(), "请求失败") {
			t.Fatalf("input was not validated: %v", err)
		}
	}
}

func TestWeatherResponseFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"http", 403, "forbidden"}, {"invalid_json", 200, "not JSON"},
		{"api_error", 200, `{"code":"401"}`}, {"empty", 200, `{"code":"200"}`},
		{"oversize", 200, strings.Repeat("x", (1<<20)+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); w.Write([]byte(tc.body)) }))
			defer s.Close()
			w := NewWeather("key", s.URL)
			if _, err := w.now(context.Background(), "123"); err == nil {
				t.Fatal("bad weather response accepted")
			}
		})
	}
}
