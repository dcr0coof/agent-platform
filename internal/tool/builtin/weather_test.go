package builtin

import (
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

func TestWeatherObservationEvidence(t *testing.T) {
	fetched := time.Date(2026, 9, 18, 4, 30, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, updated, observed, want string
	}{
		{"provider minute precision", "2026-09-18T12:27+08:00", "2026-09-18T12:22+08:00", "观测年龄：8m0s"},
		{"UTC seconds", "2026-09-18T04:29:00Z", "2026-09-18T04:20:30Z", "观测年龄：9m30s"},
		{"old observation", "2026-09-18T04:30:00Z", "2026-09-17T04:30:00Z", "观测年龄：24h0m0s"},
		{"missing observation", "2026-09-18T04:30:00Z", "", "观测时间：未提供，时间未知"},
		{"invalid observation", "", "not-a-date", "观测时间：格式无效，时间未知"},
		{"no timezone", "", "2026-09-18T04:20:00", "观测时间：格式无效，时间未知"},
		{"future observation", "", "2026-09-18T04:31:00Z", "观测时间：2026-09-18T04:31:00Z（晚于获取时间，时钟或数据异常）"},
		{"missing update", "", "2026-09-18T04:20:00Z", "API 更新时间：未提供，时间未知"},
		{"invalid update", "yesterday", "", "API 更新时间：格式无效，时间未知"},
		{"future update", "2026-09-18T05:00:00Z", "", "API 更新时间：2026-09-18T05:00:00Z（晚于获取时间，时钟或数据异常）"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := observationEvidence("123", fetched, tc.updated, tc.observed)
			for _, want := range []string{tc.want, "来源：QWeather", "Location ID：123", "获取时间：2026-09-18T04:30:00Z"} {
				if !strings.Contains(got, want) {
					t.Fatalf("missing %q: %s", want, got)
				}
			}
			if strings.Contains(got, "观测年龄：-") {
				t.Fatal("negative age must not represent freshness")
			}
			if strings.Contains(tc.name, "observation") && tc.name != "old observation" && !strings.Contains(got, "观测年龄：未知") {
				t.Fatal("invalid observation time must not become the fetch time")
			}
		})
	}
}

func TestWeatherObservationMetadataSurvivesHTTP(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/geo/v2/city/lookup" {
			w.Write([]byte(`{"code":"200","location":[{"id":"123","name":"测试城市"}]}`))
			return
		}
		if r.URL.Path != "/v7/weather/now" || r.URL.Query().Get("location") != "123" {
			t.Errorf("unexpected request %s", r.URL.String())
		}
		w.Write([]byte(`{"code":"200","updateTime":"2026-09-17T12:27+08:00","now":{"obsTime":"2026-09-17T12:22+08:00","temp":"26","text":"晴"}}`))
	}))
	defer s.Close()
	got, err := NewWeather("test-key", s.URL).Execute(context.Background(), map[string]interface{}{"city": "测试城市"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"【天气观测】", "来源：QWeather", "Location ID：123", "API 更新时间：2026-09-17T12:27+08:00", "观测时间：2026-09-17T12:22+08:00", "温度：26°C", "天气：晴"} {
		if !strings.Contains(got, want) {
			t.Fatalf("lost observation field %q: %s", want, got)
		}
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
			if kind == "forecast" && !strings.Contains(result, "未提供，无法确认数据新鲜度") {
				t.Fatal("missing provider timestamp was not disclosed")
			}
		})
	}
}

func TestWeatherRejectsBadInputBeforeNetwork(t *testing.T) {
	w := NewWeather("key", "http://127.0.0.1:1")
	for _, params := range []map[string]interface{}{
		nil, {"city": " "}, {"city": "北京", "type": "invalid"}, {"city": "北京", "type": 3},
		{"city": "北京", "date": "2026-02-29"}, {"city": "北京", "date": "2026-9-17"},
		{"city": "北京", "date": ""}, {"city": "北京", "date": 20260917},
		{"city": "北京", "date": "2026-09-17", "type": "now"},
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
