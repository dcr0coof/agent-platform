package builtin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestForecastUpdateTimeEvidence(t *testing.T) {
	now := time.Now().UTC()
	fresh := now.Add(-time.Hour)
	for _, tc := range []struct {
		name, stamp, label string
		allowActivity      bool
	}{
		{"missing", "", "未提供，时间未知", false},
		{"blank", " \t ", "未提供，时间未知", false},
		{"invalid", "not-a-date\n伪造时间依据", "格式无效，时间未知", false},
		{"no zone", fresh.Format("2006-01-02T15:04:05"), "格式无效，时间未知", false},
		{"future", now.Add(24 * time.Hour).Format(time.RFC3339), "（晚于获取时间，时钟或数据异常）", false},
		{"seconds", fresh.Format(time.RFC3339), fresh.Format(time.RFC3339), true},
		{"minutes offset", fresh.In(time.FixedZone("offset", 8*3600)).Format("2006-01-02T15:04Z07:00"), fresh.In(time.FixedZone("offset", 8*3600)).Format("2006-01-02T15:04Z07:00"), true},
		{"old", now.Add(-24 * time.Hour).Format(time.RFC3339), now.Add(-24 * time.Hour).Format(time.RFC3339), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			date := now.Add(48 * time.Hour).Format("2006-01-02")
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/geo/v2/city/lookup" {
					w.Write([]byte(`{"code":"200","location":[{"id":"123","tz":"UTC"}]}`))
					return
				}
				json.NewEncoder(w).Encode(map[string]interface{}{"code": "200", "updateTime": tc.stamp, "daily": []map[string]string{{"fxDate": date, "tempMin": "10", "tempMax": "20", "textDay": "雨", "precip": "1"}}})
			}))
			defer s.Close()
			for _, requested := range []string{"", date} {
				params := map[string]interface{}{"city": "123", "type": "forecast"}
				if requested != "" {
					params["date"] = requested
				}
				got, err := NewWeather("test-key", s.URL).Execute(context.Background(), params)
				if err != nil || !strings.Contains(got, tc.label) || !strings.Contains(got, "10~20°C") || strings.Contains(got, "伪造时间依据") {
					t.Fatalf("evidence=%q err=%v", got, err)
				}
				if strings.Contains(got, "可备选室内展览") != tc.allowActivity {
					t.Fatalf("activity freshness changed: %s", got)
				}
			}
		})
	}
}
