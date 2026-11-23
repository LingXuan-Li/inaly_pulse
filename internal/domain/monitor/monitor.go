// Package monitor は監視対象(Monitor)と個々の監視結果(Result)のドメインモデルを定義する。
// このパッケージは標準ライブラリのみに依存し、GitHub API や Discord API を参照しない。
package monitor

import "time"

// Type は監視方式の識別子であり、Checker の登録キーになる。
type Type string

const (
	TypeHTTP Type = "http"
	TypeTCP  Type = "tcp"
	TypeDNS  Type = "dns"
	TypePing Type = "ping"
)

// Spec は監視方式ごとの固有設定を表す。
// 各方式(internal/checker/http など)が自身の型で実装し、型安全に保持する。
type Spec interface {
	Type() Type
}

// LatencyPolicy は応答時間の評価条件。
// Latency は独立した監視方式ではなく、各 Checker の結果に適用する共通機能として扱う(仕様8.7)。
type LatencyPolicy struct {
	Warning             time.Duration
	Critical            time.Duration
	ConsecutiveBreaches int    // 連続超過回数
	Metric              string // 評価対象(例: "total")。取りうる値は Phase 5 で確定する
}

// Monitor は1つの監視対象の設定。
type Monitor struct {
	ID       string
	Name     string
	Type     Type
	Enabled  bool
	Target   string
	Interval time.Duration
	Timeout  time.Duration
	Latency  *LatencyPolicy
	Spec     Spec
}
