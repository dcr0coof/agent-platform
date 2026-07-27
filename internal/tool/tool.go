package tool

import "context"

// Tool 所有工具的抽象接口
type Tool interface {
	Name() string
	Description() string
	// Parameters 返回 JSON Schema 参数定义
	Parameters() map[string]interface{}
	// Execute 执行工具
	Execute(ctx context.Context, params map[string]interface{}) (string, error)
}
