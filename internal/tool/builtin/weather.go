package builtin

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
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
		"参数 type 可选 'now'（实时天气，默认）或 'forecast'（3天预报）。" +
		"查询指定日期必须传 date（YYYY-MM-DD），此时默认 forecast；只能使用返回中覆盖该日期的数据，未覆盖则天气未知。"
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
			"date": map[string]interface{}{
				"type":        "string",
				"description": "目的地的预报日期 YYYY-MM-DD；仅用于 forecast，未覆盖时返回未知。不传则返回三天接口提供的全部日期。",
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
	date := ""
	if value, exists := params["date"]; exists {
		var valid bool
		date, valid = value.(string)
		if _, err := time.Parse("2006-01-02", date); !valid || err != nil {
			return "", fmt.Errorf("date 必须是有效日期，格式为 YYYY-MM-DD")
		}
		queryType = "forecast"
	}
	if value, exists := params["type"]; exists {
		var valid bool
		queryType, valid = value.(string)
		if !valid || (queryType != "now" && queryType != "forecast") {
			return "", fmt.Errorf("type 必须是 now 或 forecast")
		}
	}
	if date != "" && queryType != "forecast" {
		return "", fmt.Errorf("指定 date 时 type 必须是 forecast，不能使用实时天气代替预报")
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
		return w.forecast(ctx, locationID, date)
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
func (w *Weather) forecast(ctx context.Context, locationID, date string) (string, error) {
	u := w.endpoint("/v7/weather/3d", locationID)

	data, err := w.doGet(ctx, u)
	if err != nil {
		return "", err
	}
	fetchedAt := time.Now().UTC().Format(time.RFC3339)

	var result struct {
		Code       string `json:"code"`
		UpdateTime string `json:"updateTime"`
		Daily      []struct {
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

	sort.Slice(result.Daily, func(i, j int) bool { return result.Daily[i].FxDate < result.Daily[j].FxDate })
	dates := make([]string, 0, len(result.Daily))
	for i, d := range result.Daily {
		if _, err := time.Parse("2006-01-02", d.FxDate); err != nil {
			return "", fmt.Errorf("预报响应包含无效日期")
		}
		if i > 0 && d.FxDate == result.Daily[i-1].FxDate {
			return "", fmt.Errorf("预报响应包含重复日期")
		}
		if strings.TrimSpace(d.TempMin) == "" || strings.TrimSpace(d.TempMax) == "" || strings.TrimSpace(d.TextDay) == "" {
			return "", fmt.Errorf("预报响应缺少温度或天气描述")
		}
		dates = append(dates, d.FxDate)
	}
	if result.UpdateTime == "" {
		result.UpdateTime = "未提供，无法确认数据新鲜度"
	}
	var output strings.Builder
	fmt.Fprintf(&output, "【3天天气预报】\n来源：QWeather\nLocation ID：%s\n获取时间：%s\n预报更新时间：%s\n可用预报日期：%s\n", locationID, fetchedAt, result.UpdateTime, strings.Join(dates, "、"))
	if date != "" {
		fmt.Fprintf(&output, "请求日期：%s\n", date)
	}
	matched := false
	for _, d := range result.Daily {
		if date != "" && date != d.FxDate {
			continue
		}
		matched = true
		fmt.Fprintf(&output, "  %s：%s，%s~%s°C，%s风%s级\n",
			d.FxDate, d.TextDay, d.TempMin, d.TempMax, d.WindDirDay, d.WindScaleDay)
	}
	if !matched {
		output.WriteString("天气未知：请求日期未被本次预报覆盖，不能用其他日期的天气推断。\n")
	}
	return output.String(), nil
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
