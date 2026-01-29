package db

import (
	"testing"
)

// 纯函数单元测试，不需要数据库连接
func TestConvertValue(t *testing.T) {
	cases := []struct {
		input any
		want  any
	}{
		{[]byte("hello"), "hello"},
		{[]byte{}, ""},
		{"string", "string"},
		{42, 42},
		{3.14, 3.14},
		{nil, nil},
		{true, true},
	}

	for _, tc := range cases {
		got := convertValue(tc.input)
		if got != tc.want {
			t.Errorf("convertValue(%v)=%v, want %v", tc.input, got, tc.want)
		}
	}
}

// Query 函数的集成测试在 tests/integration/query_test.go 中
