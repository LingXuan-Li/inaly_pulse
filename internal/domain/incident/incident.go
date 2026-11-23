// Package incident は障害(Incident)のドメインモデルを定義する。
// 障害状態の正本は Git 上の JSON であり、GitHub Issues は表示・管理手段として扱う(仕様10.3)。
package incident

import "time"

// State は障害の状態(Incident State)。
type State string

const (
	StateOpen          State = "open"
	StateInvestigating State = "investigating"
	StateResolved      State = "resolved"
)

// Valid は定義済みの値かどうかを返す。
func (s State) Valid() bool {
	switch s {
	case StateOpen, StateInvestigating, StateResolved:
		return true
	}
	return false
}

// Active は継続中の障害かどうかを返す。
func (s State) Active() bool {
	return s == StateOpen || s == StateInvestigating
}

// TrackerRef は障害管理側(GitHub Issues)の対応先。
// 特定のサービス名をドメインに持ち込まないため、Issue番号とURLのみを保持する。
type TrackerRef struct {
	Number int
	URL    string
}

// Incident は1回の障害。復旧後の再障害は新しい Incident になる。
type Incident struct {
	ID         string
	MonitorID  string
	State      State
	OpenedAt   time.Time
	ResolvedAt *time.Time
	Tracker    *TrackerRef
}
