package builtin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const dateForecast = `{"code":"200","updateTime":"2026-09-17T08:00+08:00","daily":[
{"fxDate":"2026-09-19","tempMax":"25","tempMin":"20","textDay":"小雨"},
{"fxDate":"2026-09-17","tempMax":"30","tempMin":"23","textDay":"晴"}]}`

func TestWeatherForecastDateCoverage(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/geo/v2/city/lookup" {
			w.Write([]byte(`{"code":"200","location":[{"id":"101210101","name":"杭州"}]}`))
			return
		}
		if r.URL.Path != "/v7/weather/3d" || r.URL.Query().Get("location") != "101210101" {
			t.Errorf("wrong forecast endpoint: %s", r.URL)
		}
		w.Write([]byte(dateForecast))
	}))
	defer s.Close()
	for _, tc := range []struct{ date, want, absent string }{
		{"2026-09-17", "2026-09-17：晴", "小雨"},
		{"2026-09-19", "2026-09-19：小雨", "晴"},
		{"2026-09-16", "天气未知", "°C"},
		{"2026-09-18", "天气未知", "°C"}, // A gap is not covered by min/max dates.
		{"2026-09-20", "天气未知", "°C"},
		{"2028-02-29", "天气未知", "°C"}, // Valid leap day, outside returned data.
		{"", "2026-09-19：小雨", "天气未知"},
	} {
		t.Run(tc.date, func(t *testing.T) {
			params := map[string]interface{}{"city": "杭州"}
			if tc.date != "" {
				params["date"] = tc.date // Date alone selects forecast, never now.
			} else {
				params["type"] = "forecast"
			}
			before := time.Now().UTC().Add(-time.Second)
			result, err := NewWeather("private-key", s.URL).Execute(context.Background(), params)
			if err != nil || !strings.Contains(result, tc.want) || strings.Contains(result, tc.absent) {
				t.Fatalf("date coverage failed: %q, %v", result, err)
			}
			for _, marker := range []string{"来源：QWeather", "Location ID：101210101", "预报更新时间：2026-09-17T08:00+08:00", "可用预报日期：2026-09-17、2026-09-19"} {
				if !strings.Contains(result, marker) {
					t.Errorf("missing evidence %q in %s", marker, result)
				}
			}
			_, fetched, found := strings.Cut(result, "获取时间：")
			stamp, err := time.Parse(time.RFC3339, strings.Split(fetched, "\n")[0])
			if !found || err != nil || stamp.Before(before) || stamp.After(time.Now().UTC()) {
				t.Fatalf("invalid retrieval timestamp: %q", fetched)
			}
		})
	}
}

func TestWeatherRejectsInvalidForecastEvidence(t *testing.T) {
	for _, body := range []string{
		strings.Replace(dateForecast, "2026-09-19", "2026-02-30", 1),
		strings.Replace(dateForecast, "2026-09-19", "2026-09-17", 1),
		strings.Replace(dateForecast, `"tempMax":"25"`, `"tempMax":""`, 1),
		strings.Replace(dateForecast, `"textDay":"小雨"`, `"textDay":""`, 1),
		`{"code":"200","daily":[]}`, `{"code":"503"}`, `not JSON`,
	} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/geo/v2/city/lookup" {
				w.Write([]byte(`{"code":"200","location":[{"id":"123"}]}`))
				return
			}
			w.Write([]byte(body))
		}))
		_, err := NewWeather("key", s.URL).Execute(context.Background(), map[string]interface{}{"city": "杭州", "type": "forecast", "date": "2026-09-19"})
		s.Close()
		if err == nil {
			t.Fatalf("invalid forecast accepted: %s", body)
		}
	}
}

func TestWeatherDatedRequestCancellation(t *testing.T) {
	reached := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/geo/v2/city/lookup" {
			w.Write([]byte(`{"code":"200","location":[{"id":"123"}]}`))
			return
		}
		close(reached)
		<-r.Context().Done()
	}))
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := NewWeather("key", s.URL).Execute(ctx, map[string]interface{}{"city": "杭州", "date": "2026-09-19"})
		done <- err
	}()
	select {
	case <-reached:
	case <-ctx.Done():
		t.Fatal("forecast request did not arrive")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation was not preserved: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("request ignored cancellation")
	}
}
