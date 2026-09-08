package builtin

import (
	"context"
	"testing"
)

func TestCalculator_Execute(t *testing.T) {
	calc := &Calculator{}

	tests := []struct {
		expr   string
		expect string
	}{
		{"2+3", "2+3 = 5"},
		{"10-4", "10-4 = 6"},
		{"3*7", "3*7 = 21"},
		{"8/2", "8/2 = 4"},
		{"(1+2)*3", "(1+2)*3 = 9"},
		{"2.5+3.5", "2.5+3.5 = 6"},
	}

	for _, tc := range tests {
		result, err := calc.Execute(context.Background(), map[string]interface{}{
			"expression": tc.expr,
		})
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.expr, err)
			continue
		}
		if result != tc.expect {
			t.Errorf("%s: expected %q, got %q", tc.expr, tc.expect, result)
		}
	}
}

func TestCalculator_DivideByZero(t *testing.T) {
	calc := &Calculator{}
	_, err := calc.Execute(context.Background(), map[string]interface{}{
		"expression": "1/0",
	})
	if err == nil {
		t.Fatal("expected error for division by zero")
	}
}

func TestCalculatorRejectsInvalidArithmetic(t *testing.T) {
	for _, expr := range []string{"!1", "^1", "&1", "1e308*1e308"} {
		t.Run(expr, func(t *testing.T) {
			_, err := (&Calculator{}).Execute(context.Background(), map[string]interface{}{"expression": expr})
			if err == nil {
				t.Fatal("accepted unsupported or non-finite arithmetic")
			}
		})
	}
}

func TestDateTime_Execute(t *testing.T) {
	dt := &DateTime{}
	result, err := dt.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("DateTime failed: %v", err)
	}
	if result == "" {
		t.Fatal("empty result")
	}
	if len(result) < 10 {
		t.Errorf("result too short: %s", result)
	}
}
