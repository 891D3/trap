// conv.go：查询结果到 Go 值的转换助手与时间格式常量。
//
// mygo sqlite 的查询结果里每列是 any（nil/int64/float64/string/[]byte），
// 这里集中做类型收窄，避免各表代码重复书写断言。
package store

import (
	"errors"
	"time"
)

// timeRFC3339 是时间字段的统一存储格式：UTC RFC3339（AGENTS.md §4.1）。
const timeRFC3339 = time.RFC3339

// ErrNotFound 是「记录不存在」的统一错误；可用 errors.Is 判断。
// 供各表存取代码共同使用（v2 前位于 todos.go，随重写移至本文件集中导出）。
var ErrNotFound = errors.New("store: 记录不存在")

// columnText 断言列为 TEXT；非 string（含 NULL）返回空串。
func columnText(v any) string {
	s, _ := v.(string)
	return s
}

// columnTextOpt 断言列为可空 TEXT；NULL 返回 nil，非 string 同样视为 NULL。
func columnTextOpt(v any) *string {
	if s, ok := v.(string); ok {
		return &s
	}
	return nil
}

// columnInt 断言列为 INTEGER；非 int64 返回 0。
func columnInt(v any) int64 {
	n, _ := v.(int64)
	return n
}

// columnBool 断言列为 0/1 INTEGER；约定非 1 一律为 false。
func columnBool(v any) bool {
	return columnInt(v) == 1
}
