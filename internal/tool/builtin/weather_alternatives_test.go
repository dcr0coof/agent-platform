package builtin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPrecipitationAlternative(t *testing.T) {
	now := time.Date(2026, 9, 22, 2, 30, 0, 0, time.UTC)
	for _, tc := range []struct{ name, precip, updated, want string }{
		{"wet", "4.1", "2026-09-22T10:00+08:00", "室内展览或室内阅读休息"},
		{"zero", "0", "2026-09-22T02:30:00Z", "不据此认定适合户外"},
		{"boundary", "1", "2026-09-21T20:30:00Z", "室内展览"},
		{"old", "1", "2026-09-21T20:29:59Z", "超过应用 6 小时"},
		{"missing time", "1", "", "更新时间未知或异常"},
		{"invalid time", "1", "not-a-time", "更新时间未知或异常"},
		{"future time", "1", "2026-09-22T02:30:01Z", "更新时间未知或异常"},
		{"missing precip", "", "2026-09-22T02:30:00Z", "降水依据缺失或无效"},
		{"negative", "-1", "2026-09-22T02:30:00Z", "降水依据缺失或无效"},
		{"text", "小雨", "2026-09-22T02:30:00Z", "降水依据缺失或无效"},
		{"nan", "NaN", "2026-09-22T02:30:00Z", "降水依据缺失或无效"},
		{"infinity", "+Inf", "2026-09-22T02:30:00Z", "降水依据缺失或无效"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := precipitationAlternative(tc.precip, tc.updated, now, "2026-09-22", "2026-09-22")
			if !strings.Contains(got, tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			if tc.name != "wet" && tc.name != "boundary" && strings.Contains(got, "室内展览") {
				t.Fatal("unsupported activity recommendation")
			}
		})
	}
}

func TestForecastAlternativesStayWithRequestedDate(t *testing.T) {
	date := time.Now().UTC().AddDate(0, 0, 1)
	wetDate, dryDate, missingDate := date.Format("2006-01-02"), date.AddDate(0, 0, 1).Format("2006-01-02"), date.AddDate(0, 0, 2).Format("2006-01-02")
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/geo/v2/city/lookup" {
			fmt.Fprint(w, `{"code":"200","location":[{"id":"123","tz":"UTC"}]}`)
			return
		}
		fmt.Fprintf(w, `{"code":"200","updateTime":%q,"daily":[{"fxDate":%q,"textDay":"雨","tempMin":"20","tempMax":"25","precip":"4.1"},{"fxDate":%q,"textDay":"晴","tempMin":"20","tempMax":"25","precip":"0"}]}`, time.Now().UTC().Add(-time.Minute).Format(time.RFC3339), wetDate, dryDate)
	}))
	defer s.Close()
	for _, tc := range []struct{ date, want, absent string }{
		{wetDate, "预报降水量：4.1 mm", "未预报降水"},
		{dryDate, "不据此认定适合户外", "室内展览"},
		{missingDate, "天气未知", "活动备选"},
	} {
		got, err := NewWeather("test", s.URL).Execute(context.Background(), map[string]interface{}{"city": "测试", "date": tc.date})
		if err != nil || !strings.Contains(got, tc.want) || strings.Contains(got, tc.absent) {
			t.Fatalf("wrong date alternative: %s %v", got, err)
		}
	}
}
