package builtin

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // Keep destination timezone support in standalone Windows binaries.
)

// This is an application planning threshold, not a provider freshness guarantee.
const forecastPlanningMaxAge = 6 * time.Hour

func destinationDate(zone string, now time.Time) string {
	// LoadLocation treats empty/Local as machine settings; neither identifies a destination.
	if zone == "" || zone == "Local" {
		return ""
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		return ""
	}
	return now.In(location).Format("2006-01-02")
}

func precipitationAlternative(precip, updateTime string, fetchedAt time.Time, forecastDate, localDate string) string {
	amount, err := strconv.ParseFloat(strings.TrimSpace(precip), 64)
	if err != nil || math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 {
		return "预报降水量：未知；活动备选：降水依据缺失或无效，暂不生成。"
	}
	evidence := fmt.Sprintf("预报降水量：%g mm；", amount)
	if localDate == "" {
		return evidence + "活动备选：目的地当地日期未知，暂不生成。"
	}
	// forecastDate has already been validated as YYYY-MM-DD by forecast.
	if forecastDate < localDate {
		return evidence + "活动备选：该日期在目的地已过去，仅保留历史预报记录，不生成新活动建议。"
	}
	_, updated := providerTimeLabel(updateTime, fetchedAt)
	if updated.IsZero() || updated.After(fetchedAt) {
		return evidence + "活动备选：预报更新时间未知或异常，暂不生成。"
	}
	if fetchedAt.Sub(updated) > forecastPlanningMaxAge {
		return evidence + "活动备选：预报已超过应用 6 小时规划窗口，请刷新后再生成。"
	}
	if amount == 0 {
		return evidence + "活动备选（应用规则）：该记录未预报降水，不据此认定适合户外；仍需核对温度、风力与预警。"
	}
	return evidence + "活动备选（应用规则，非气象观测）：若原计划包含露天活动，可备选室内展览或室内阅读休息。仅为活动类别，需结合个人偏好并核对开放、交通与预警；不保证适合出行。"
}
