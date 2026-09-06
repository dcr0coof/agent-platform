// Package trip implements persisted trip conversations and observable runs.
package trip

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/dcr0coof/agent-platform/internal/llm"
)

type Constraints struct {
	Origin      string  `json:"origin"`
	Destination string  `json:"destination"`
	StartDate   string  `json:"start_date"`
	EndDate     string  `json:"end_date"`
	Travelers   int     `json:"travelers"`
	Budget      float64 `json:"budget"`
	BudgetScope string  `json:"budget_scope"`
	Preferences string  `json:"preferences"`
}

func (c Constraints) Validate() error {
	if len(c.Origin) > 200 || len(c.Destination) > 200 || len(c.Preferences) > 4000 {
		return fmt.Errorf("出发地、目的地或偏好过长")
	}
	if c.Travelers < 0 || c.Travelers > 100 {
		return fmt.Errorf("人数须为 1–100，未知时填 0")
	}
	if math.IsNaN(c.Budget) || math.IsInf(c.Budget, 0) || c.Budget < 0 || c.Budget > 10000000 {
		return fmt.Errorf("预算须在 0–10000000 元之间")
	}
	if c.BudgetScope != "" && c.BudgetScope != "all" && c.BudgetScope != "local" {
		return fmt.Errorf("预算范围须为 all 或 local")
	}
	for _, d := range []string{c.StartDate, c.EndDate} {
		if d != "" {
			if _, err := time.Parse("2006-01-02", d); err != nil {
				return fmt.Errorf("日期格式须为 YYYY-MM-DD")
			}
		}
	}
	if c.StartDate != "" && c.EndDate != "" && c.StartDate > c.EndDate {
		return fmt.Errorf("结束日期不能早于开始日期")
	}
	return nil
}

func (c Constraints) Missing() []string {
	var missing []string
	if strings.TrimSpace(c.Origin) == "" {
		missing = append(missing, "出发地")
	}
	if strings.TrimSpace(c.Destination) == "" {
		missing = append(missing, "目的地")
	}
	if c.StartDate == "" || c.EndDate == "" {
		missing = append(missing, "出行日期")
	}
	if c.Travelers == 0 {
		missing = append(missing, "出行人数")
	}
	if c.Budget == 0 {
		missing = append(missing, "预算金额")
	}
	if c.BudgetScope == "" {
		missing = append(missing, "预算是否包含往返交通和住宿")
	}
	return missing
}

type Session struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Constraints Constraints   `json:"constraints"`
	Messages    []llm.Message `json:"messages"`
	Revision    int           `json:"revision"`
	CreatedAt   string        `json:"created_at"`
	UpdatedAt   string        `json:"updated_at"`
	ActiveRun   *Run          `json:"active_run,omitempty"`
}

type Event struct {
	Seq       int    `json:"seq"`
	Type      string `json:"type"`
	Detail    string `json:"detail"`
	CreatedAt string `json:"created_at"`
}

type Run struct {
	ID        string  `json:"id"`
	SessionID string  `json:"session_id"`
	RequestID string  `json:"request_id"`
	Input     string  `json:"input"`
	Status    string  `json:"status"`
	Error     string  `json:"error,omitempty"`
	CreatedAt string  `json:"created_at"`
	Events    []Event `json:"events"`
}

func (r Run) Terminal() bool {
	return r.Status == "completed" || r.Status == "failed" || r.Status == "cancelled"
}
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func timestamp() string { return time.Now().UTC().Format(time.RFC3339Nano) }

var ErrNotFound = fmt.Errorf("会话或运行不存在")
var ErrConflict = fmt.Errorf("会话已变化或正在运行，请刷新后重试")
