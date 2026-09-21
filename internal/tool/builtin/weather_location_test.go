package builtin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWeatherLocationSelection(t *testing.T) {
	const candidates = `[{"id":"123","name":"朝阳","adm2":"北京","adm1":"北京","country":"中国"},{"id":"456","name":"朝阳","adm2":"朝阳","adm1":"辽宁","country":"中国"}]`
	for _, kind := range []string{"now", "forecast"} {
		for _, tc := range []struct {
			name, city, locations, wantID, wantError string
		}{
			{"ambiguous", "朝阳", candidates, "", "请用户选择"},
			{"exact second ID", "456", candidates, "456", ""},
			{"unknown ID cannot pick first", "999", candidates, "", "请用户选择"},
			{"single", "朝阳", `[{"id":"123","name":"朝阳"}]`, "123", ""},
			{"duplicate ID", "朝阳", `[{"id":"123"},{"id":"123"}]`, "123", ""},
			{"missing ID", "朝阳", `[{"id":"123"},{"name":"朝阳"}]`, "", "缺少地点 ID"},
			{"blank ID", "朝阳", `[{"id":"  "}]`, "", "缺少地点 ID"},
		} {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				weatherCalls := 0
				s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/geo/v2/city/lookup" {
						if r.URL.Query().Get("location") != tc.city {
							t.Errorf("lookup query changed: %s", r.URL.RawQuery)
						}
						w.Write([]byte(`{"code":"200","location":` + tc.locations + `}`))
						return
					}
					weatherCalls++
					if r.URL.Query().Get("location") != tc.wantID {
						t.Errorf("queried wrong location: %s", r.URL.String())
					}
					w.Write([]byte(`{"code":"200","now":{"temp":"26","text":"晴"},"daily":[{"fxDate":"2026-09-21","tempMin":"20","tempMax":"28","textDay":"晴"}]}`))
				}))
				defer s.Close()
				got, err := NewWeather("test-key", s.URL).Execute(context.Background(), map[string]interface{}{"city": tc.city, "type": kind})
				if tc.wantError != "" {
					if err == nil || !strings.Contains(err.Error(), tc.wantError) || got != "" || weatherCalls != 0 {
						t.Fatalf("unsafe selection: output=%q error=%v weatherCalls=%d", got, err, weatherCalls)
					}
					if tc.name == "ambiguous" {
						for _, want := range []string{"朝阳", "北京", "辽宁", "中国", "123", "456", "city"} {
							if !strings.Contains(err.Error(), want) {
								t.Errorf("missing candidate evidence %q: %v", want, err)
							}
						}
					}
				} else if err != nil || weatherCalls != 1 || !strings.Contains(got, "Location ID："+tc.wantID) {
					t.Fatalf("selection failed: output=%q error=%v weatherCalls=%d", got, err, weatherCalls)
				}
			})
		}
	}
}
