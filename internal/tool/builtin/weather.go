package builtin

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Weather 和风天气查询工具
type Weather struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewWeather 创建天气工具
// apiKey: 和风天气 API Key
func NewWeather(apiKey, baseURL string) *Weather {
	return &Weather{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client:  &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
	}
}

func (w *Weather) Name() string { return "weather" }

func (w *Weather) Description() string {
	return "查询指定城市的实时天气或未来天气预报。" +
		"参数 city 可以是城市中文名（如'北京'、'上海'）或城市 ID。" +
		"参数 type 可选 'now'（实时天气，默认）或 'forecast'（3天预报）。"
}

func (w *Weather) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"city": map[string]interface{}{
				"type":        "string",
				"description": "城市名称，如'北京'、'上海'、'深圳'",
			},
			"type": map[string]interface{}{
				"type":        "string",
				"description": "查询类型：now（实时天气）或 forecast（3天预报）",
				"enum":        []string{"now", "forecast"},
			},
		},
		"required": []string{"city"},
	}
}

func (w *Weather) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	city, ok := params["city"].(string)
	city = strings.TrimSpace(city)
	if !ok || city == "" {
		return "", fmt.Errorf("请提供城市名称")
	}

	queryType := "now"
	if value, exists := params["type"]; exists {
		var valid bool
		queryType, valid = value.(string)
		if !valid || (queryType != "now" && queryType != "forecast") {
			return "", fmt.Errorf("type 必须是 now 或 forecast")
		}
	}
	if w.baseURL == "" {
		return "", fmt.Errorf("请配置和风天气专属 API Host（AGENT_WEATHER_BASE_URL）")
	}

	// 1. 城市搜索 → location ID
	locationID, err := w.cityLookup(ctx, city)
	if err != nil {
		return "", fmt.Errorf("城市查询失败: %w", err)
	}

	// 2. 查询天气
	switch queryType {
	case "forecast":
		return w.forecast(ctx, locationID)
	default:
		return w.now(ctx, locationID)
	}
}

// cityLookup 城市搜索，返回 LocationID
func (w *Weather) cityLookup(ctx context.Context, city string) (string, error) {
	u := w.endpoint("/geo/v2/city/lookup", city)

	data, err := w.doGet(ctx, u)
	if err != nil {
		return "", err
	}

	var result struct {
		Code     string `json:"code"`
		Location []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"location"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("解析城市数据失败: %w", err)
	}

	if result.Code == "403" {
		return "", fmt.Errorf("天气 API 拒绝访问（403），请检查 API Host、凭证和订阅权限")
	}
	if result.Code != "200" || len(result.Location) == 0 || result.Location[0].ID == "" {
		return "", fmt.Errorf("未找到城市 '%s'（code=%s）", city, result.Code)
	}

	return result.Location[0].ID, nil
}

// now 实时天气
func (w *Weather) now(ctx context.Context, locationID string) (string, error) {
	u := w.endpoint("/v7/weather/now", locationID)

	data, err := w.doGet(ctx, u)
	if err != nil {
		return "", err
	}

	var result struct {
		Code string `json:"code"`
		Now  struct {
			Temp      string `json:"temp"`
			FeelsLike string `json:"feelsLike"`
			Text      string `json:"text"`
			WindDir   string `json:"windDir"`
			WindScale string `json:"windScale"`
			Humidity  string `json:"humidity"`
		} `json:"now"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("解析天气数据失败: %w", err)
	}

	if result.Code != "200" {
		return "", fmt.Errorf("天气查询失败（code=%s），请确认 Key 是否已激活实时天气 API", result.Code)
	}
	if result.Now.Temp == "" || result.Now.Text == "" {
		return "", fmt.Errorf("天气响应缺少温度或天气描述")
	}

	return fmt.Sprintf("【实时天气】温度：%s°C（体感 %s°C），天气：%s，风向：%s，风力：%s级，湿度：%s%%",
		result.Now.Temp, result.Now.FeelsLike, result.Now.Text,
		result.Now.WindDir, result.Now.WindScale, result.Now.Humidity), nil
}

// forecast 3天预报
func (w *Weather) forecast(ctx context.Context, locationID string) (string, error) {
	u := w.endpoint("/v7/weather/3d", locationID)

	data, err := w.doGet(ctx, u)
	if err != nil {
		return "", err
	}

	var result struct {
		Code  string `json:"code"`
		Daily []struct {
			FxDate       string `json:"fxDate"`
			TempMax      string `json:"tempMax"`
			TempMin      string `json:"tempMin"`
			TextDay      string `json:"textDay"`
			WindDirDay   string `json:"windDirDay"`
			WindScaleDay string `json:"windScaleDay"`
		} `json:"daily"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("解析预报数据失败: %w", err)
	}

	if result.Code != "200" || len(result.Daily) == 0 {
		return "", fmt.Errorf("预报查询失败（code=%s）", result.Code)
	}

	output := "【3天天气预报】\n"
	for _, d := range result.Daily {
		output += fmt.Sprintf("  %s：%s，%s~%s°C，%s风%s级\n",
			d.FxDate, d.TextDay, d.TempMin, d.TempMax, d.WindDirDay, d.WindScaleDay)
	}
	return output, nil
}

func (w *Weather) doGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-QW-Api-Key", w.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("天气 HTTP %d (%s)", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	// 默认 Transport 自动解压；同时兼容主动压缩响应及自定义 Transport。
	var reader io.Reader = resp.Body
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("解压天气响应失败: %w", err)
		}
		defer gz.Close()
		reader = gz
	}

	const maxResponseBytes = 1 << 20
	data, err := io.ReadAll(io.LimitReader(reader, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	if len(data) > maxResponseBytes {
		return nil, fmt.Errorf("天气响应超过 %d 字节", maxResponseBytes)
	}
	return data, nil
}

func (w *Weather) endpoint(path, location string) string {
	return w.baseURL + path + "?location=" + strings.ReplaceAll(url.QueryEscape(location), "+", "%20")
}
