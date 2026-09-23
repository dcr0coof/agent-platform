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

func TestForecastPlanningRequiresCurrentLocalDate(t *testing.T) {
	for _, tc := range []struct{ name, zone, date string }{
		{"past", "Asia/Tokyo", "2000-01-01"},
		{"missing timezone", "", "2099-01-01"},
		{"invalid timezone", "Unknown/Place", "2099-01-01"},
		{"machine timezone is not destination", "Local", "2099-01-01"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/geo/v2/city/lookup" {
					fmt.Fprintf(w, `{"code":"200","location":[{"id":"123","tz":%q}]}`, tc.zone)
					return
				}
				fmt.Fprintf(w, `{"code":"200","updateTime":%q,"daily":[{"fxDate":%q,"textDay":"雨","tempMin":"20","tempMax":"25","precip":"4.1"}]}`, time.Now().UTC().Add(-time.Minute).Format(time.RFC3339), tc.date)
			}))
			defer s.Close()
			got, err := NewWeather("test", s.URL).Execute(context.Background(), map[string]interface{}{"city": "测试", "date": tc.date})
			if err != nil || !strings.Contains(got, "预报降水量：4.1 mm") || strings.Contains(got, "室内展览") {
				t.Fatalf("unsupported activity suggestion or evidence lost: %s, %v", got, err)
			}
		})
	}
}

func TestDestinationDateAndActivityBoundary(t *testing.T) {
	for _, tc := range []struct {
		instant, zone, wantDate, forecast string
		recommend                         bool
	}{
		{"2026-09-23T00:30:00Z", "America/Los_Angeles", "2026-09-22", "2026-09-22", true},
		{"2026-09-22T16:30:00Z", "Asia/Shanghai", "2026-09-23", "2026-09-22", false},
		{"2026-09-22T16:30:00Z", "Asia/Shanghai", "2026-09-23", "2026-09-23", true},
		{"2026-09-22T16:30:00Z", "Asia/Shanghai", "2026-09-23", "2026-09-24", true},
		{"2026-07-01T04:30:00Z", "America/New_York", "2026-07-01", "2026-06-30", false},
		{"2026-01-01T04:30:00Z", "America/New_York", "2025-12-31", "2025-12-31", true},
		{"2026-09-23T00:00:00Z", "UTC", "2026-09-23", "2026-09-23", true},
		{"2026-09-23T00:00:00Z", "", "", "2026-09-23", false},
		{"2026-09-23T00:00:00Z", "Local", "", "2026-09-23", false},
		{"2026-09-23T00:00:00Z", "Unknown/Place", "", "2026-09-23", false},
	} {
		now, err := time.Parse(time.RFC3339, tc.instant)
		if err != nil {
			t.Fatal(err)
		}
		localDate := destinationDate(tc.zone, now)
		if localDate != tc.wantDate {
			t.Errorf("%s at %s: got %q want %q", tc.zone, tc.instant, localDate, tc.wantDate)
		}
		got := precipitationAlternative("1", tc.instant, now, tc.forecast, localDate)
		if strings.Contains(got, "室内展览") != tc.recommend {
			t.Errorf("%s: %s", tc.zone, got)
		}
	}
}

func TestCitySelectionKeepsSelectedTimezone(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"code":"200","location":[{"id":"123","tz":"Asia/Shanghai"},{"id":"456","tz":"America/New_York"}]}`)
	}))
	defer s.Close()
	got, err := NewWeather("test", s.URL).cityLookup(context.Background(), "456")
	if err != nil || got.ID != "456" || got.Timezone != "America/New_York" {
		t.Fatalf("wrong location context: %+v %v", got, err)
	}
}
