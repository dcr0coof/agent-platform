package builtin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOptionalWeatherNumbersInToolOutput(t *testing.T) {
	for _, tc := range []struct{ feels, humidity, wantFeels, wantHumidity string }{
		{"", " ", "未知（未提供）", "未知（未提供）"},
		{"NaN", "NaN", "未知（数值无效）", "未知（数值无效）"},
		{"+Inf", "-Inf", "未知（数值无效）", "未知（数值无效）"},
		{"private-invalid", "101", "未知（数值无效）", "未知（数值无效）"},
		{"1e999", "-1", "未知（数值无效）", "未知（数值无效）"},
		{" -12.5 ", "0", "-12.5°C", "0%"},
		{"0", "100", "0°C", "100%"},
		{"25", " 50.5 ", "25°C", "50.5%"},
	} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/geo/v2/city/lookup" {
				w.Write([]byte(`{"code":"200","location":[{"id":"123"}]}`))
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"code": "200", "now": map[string]string{
				"temp": "20", "text": "晴", "feelsLike": tc.feels, "humidity": tc.humidity,
			}})
		}))
		result, err := NewWeather("test-key", s.URL).Execute(context.Background(), map[string]interface{}{"city": "杭州"})
		s.Close()
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"温度：20°C", "体感 " + tc.wantFeels, "湿度：" + tc.wantHumidity, "来源：QWeather", "风力：未知（未提供）"} {
			if !strings.Contains(result, want) {
				t.Fatalf("missing %s in %s", want, result)
			}
		}
		for _, invalid := range []string{"NaN", "Inf", "private-invalid", "1e999", "101%", "-1%"} {
			if strings.Contains(result, invalid) {
				t.Fatalf("invalid evidence exposed: %s", result)
			}
		}
	}
}

func TestForecastOptionalWindInToolOutput(t *testing.T) {
	for _, tc := range []struct{ dir, scale, wantDir, wantScale string }{
		{"", " ", "未知（未提供）", "未知（未提供）"},
		{" 东北 ", "", "东北", "未知（未提供）"},
		{"", "0", "未知（未提供）", "0级"},
		{"北", " 3-4 ", "北", "3-4级"},
	} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/geo/v2/city/lookup" {
				w.Write([]byte(`{"code":"200","location":[{"id":"123"}]}`))
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"code": "200", "daily": []map[string]string{{
				"fxDate": "2026-10-19", "tempMin": "10", "tempMax": "20", "textDay": "晴", "windDirDay": tc.dir, "windScaleDay": tc.scale,
			}}})
		}))
		result, err := NewWeather("test-key", s.URL).Execute(context.Background(), map[string]interface{}{"city": "杭州", "date": "2026-10-19"})
		s.Close()
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"2026-10-19：晴，10~20°C", "风向：" + tc.wantDir, "风力：" + tc.wantScale} {
			if !strings.Contains(result, want) {
				t.Fatalf("missing %s in %s", want, result)
			}
		}
	}
}
