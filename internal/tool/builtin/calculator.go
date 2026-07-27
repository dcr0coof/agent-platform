package builtin

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"strconv"
)

// Calculator 简单表达式求值计算器
type Calculator struct{}

func (c *Calculator) Name() string        { return "calculator" }
func (c *Calculator) Description() string { return "计算数学表达式，支持 + - * / 和括号，如 '2+3*4' 或 '(1+2)*3'" }
func (c *Calculator) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"expression": map[string]interface{}{
				"type":        "string",
				"description": "要计算的数学表达式",
			},
		},
		"required": []string{"expression"},
	}
}

func (c *Calculator) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	expr, ok := params["expression"].(string)
	if !ok {
		return "", fmt.Errorf("expression is required")
	}
	result, err := evalExpr(expr)
	if err != nil {
		return "", fmt.Errorf("计算失败: %w", err)
	}
	// 整数结果不显示小数点
	if result == math.Trunc(result) {
		return fmt.Sprintf("%s = %.0f", expr, result), nil
	}
	return fmt.Sprintf("%s = %g", expr, result), nil
}

// evalExpr 用 Go parser 安全求值
// 只支持四则运算和括号，不执行任何函数调用
func evalExpr(expr string) (float64, error) {
	node, err := parser.ParseExpr(expr)
	if err != nil {
		return 0, fmt.Errorf("表达式语法错误: %s", expr)
	}
	return eval(node)
}

func eval(node ast.Expr) (float64, error) {
	switch n := node.(type) {
	case *ast.BasicLit:
		if n.Kind == token.INT || n.Kind == token.FLOAT {
			return strconv.ParseFloat(n.Value, 64)
		}
		return 0, fmt.Errorf("不支持的类型: %s", n.Value)
	case *ast.BinaryExpr:
		left, err := eval(n.X)
		if err != nil {
			return 0, err
		}
		right, err := eval(n.Y)
		if err != nil {
			return 0, err
		}
		switch n.Op {
		case token.ADD:
			return left + right, nil
		case token.SUB:
			return left - right, nil
		case token.MUL:
			return left * right, nil
		case token.QUO:
			if right == 0 {
				return 0, fmt.Errorf("除数不能为 0")
			}
			return left / right, nil
		default:
			return 0, fmt.Errorf("不支持的运算符: %s", n.Op)
		}
	case *ast.ParenExpr:
		return eval(n.X)
	case *ast.UnaryExpr:
		val, err := eval(n.X)
		if err != nil {
			return 0, err
		}
		if n.Op == token.SUB {
			return -val, nil
		}
		return val, nil
	default:
		return 0, fmt.Errorf("不支持的表达式类型")
	}
}
