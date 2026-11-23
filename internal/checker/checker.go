// Package checker は監視方式(HTTP / TCP / DNS / Ping)の共通インターフェースと登録機構を定義する。
// 各方式は internal/checker/<方式>/ に実装し、Registry に登録する。
package checker

import (
	"context"

	"inaly_pulse/internal/domain/monitor"
)

// Checker は1つの監視方式を実装する。
// Check は失敗を error ではなく Result.Status と Result.Error で返す。
// これにより、監視対象の失敗と Bot 自身の内部エラーを区別する。
type Checker interface {
	Type() monitor.Type
	Check(ctx context.Context, m monitor.Monitor) monitor.Result
}
