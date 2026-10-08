package builtin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWeatherRejectsInvalidTemperatureEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, mode, min, max, text string
		valid                      bool
	}{
		{"now nan", "now", "NaN", "", "晴", false},
		{"now infinity", "now", "+Inf", "", "晴", false},
		{"now overflow", "now", "1e999", "", "晴", false},
		{"now text", "now", "unknown", "", "晴", false},
		{"now blank", "now", " ", "", "晴", false},
		{"now blank description", "now", "0", "", "\t", false},
		{"now zero", "now", "0", "", "晴", true},
		{"now negative", "now", "-2.5", "", "晴", true},
		{"forecast nan min", "forecast", "NaN", "20", "晴", false},
		{"forecast infinite max", "forecast", "0", "Inf", "晴", false},
		{"forecast text max", "forecast", "0", "unknown", "晴", false},
		{"forecast reversed", "forecast", "20", "10", "晴", false},
		{"forecast blank", "forecast", " ", "10", "晴", false},
		{"forecast blank description", "forecast", "0", "10", " ", false},
		{"forecast zero", "forecast", "0", "0", "晴", true},
		{"forecast negative", "forecast", "-10.5", " -2 ", "晴", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload interface{}
				if r.URL.Path == "/geo/v2/city/lookup" {
					payload = map[string]interface{}{"code": "200", "location": []map[string]string{{"id": "123", "tz": "Asia/Shanghai"}}}
				} else if tc.mode == "now" {
					payload = map[string]interface{}{"code": "200", "now": map[string]string{"temp": tc.min, "text": tc.text}}
				} else {
					payload = map[string]interface{}{"code": "200", "daily": []map[string]string{{"fxDate": "2026-10-08", "tempMin": tc.min, "tempMax": tc.max, "textDay": tc.text, "precip": "1"}}}
				}
				json.NewEncoder(w).Encode(payload)
			}))
			defer s.Close()
			got, err := NewWeather("test-key", s.URL).Execute(context.Background(), map[string]interface{}{"city": "123", "type": tc.mode})
			if tc.valid {
				if err != nil || !strings.Contains(got, "来源：QWeather") || !strings.Contains(got, tc.min) {
					t.Fatalf("valid temperature rejected: %s, %v", got, err)
				}
			} else if err == nil || got != "" || (!strings.Contains(err.Error(), "温度") && !strings.Contains(err.Error(), "天气描述")) {
				t.Fatalf("invalid response became evidence: %s, %v", got, err)
			}
		})
	}
}
