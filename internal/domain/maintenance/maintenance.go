// Package maintenance はメンテナンス期間のドメインモデルを定義する。
// pause(監視自体の無効化)とは別機能であり、このパッケージでは扱わない。
package maintenance

import "time"

// Scope はメンテナンスの適用範囲(Global / Monitor)。
type Scope string

const (
	ScopeGlobal  Scope = "global"
	ScopeMonitor Scope = "monitor"
)

// Origin はメンテナンスの設定元(Scheduled / Manual)。
type Origin string

const (
	OriginScheduled Origin = "scheduled"
	OriginManual    Origin = "manual"
)

// Mode はメンテナンス中の動作モード。
type Mode string

const (
	// ModeSkip は監視処理自体を実行しない。
	ModeSkip Mode = "skip"
	// ModeSuppress は監視を継続し、通知と新規障害Issueの作成を抑制する。
	ModeSuppress Mode = "suppress"
)

// DefaultMode は仕様11.2の既定値。
const DefaultMode = ModeSuppress

// State はメンテナンスの状態(Maintenance State)。
type State string

const (
	StateInactive State = "inactive"
	StateActive   State = "active"
)

// Window は具体的な1つのメンテナンス期間。
// 繰り返し予定(recurring)の表現は未確定のため、このモデルでは固定期間のみを扱う。
type Window struct {
	ID        string
	Scope     Scope
	MonitorID string // Scope が monitor の場合のみ使用
	Origin    Origin
	Mode      Mode
	StartsAt  time.Time
	EndsAt    time.Time
}

// StateAt は時刻 t における状態を返す。開始時刻を含み、終了時刻は含まない。
func (w Window) StateAt(t time.Time) State {
	if !t.Before(w.StartsAt) && t.Before(w.EndsAt) {
		return StateActive
	}
	return StateInactive
}

// AppliesTo はこの期間が指定の監視対象に適用されるかを返す。
func (w Window) AppliesTo(monitorID string) bool {
	switch w.Scope {
	case ScopeGlobal:
		return true
	case ScopeMonitor:
		return w.MonitorID == monitorID
	}
	return false
}
