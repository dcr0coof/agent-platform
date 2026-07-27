package builtin

import (
	"context"
	"fmt"
	"time"
)

// DateTime 获取当前日期时间
type DateTime struct{}

func (d *DateTime) Name() string        { return "datetime" }
func (d *DateTime) Description() string { return "获取当前日期和时间" }
func (d *DateTime) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (d *DateTime) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	now := time.Now()
	return fmt.Sprintf("当前时间：%s（星期%s）",
		now.Format("2006年1月2日 15:04:05"),
		[]string{"日", "一", "二", "三", "四", "五", "六"}[now.Weekday()]), nil
}
