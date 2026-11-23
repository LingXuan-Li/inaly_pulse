package monitor

// CheckStatus は個々の監視実行の結果(Check Result)。Monitor State とは別物。
type CheckStatus string

const (
	CheckSuccess     CheckStatus = "success"
	CheckFailure     CheckStatus = "failure"
	CheckTimeout     CheckStatus = "timeout"
	CheckUnsupported CheckStatus = "unsupported"
	CheckSkipped     CheckStatus = "skipped"
)

// Valid は定義済みの値かどうかを返す。
func (s CheckStatus) Valid() bool {
	switch s {
	case CheckSuccess, CheckFailure, CheckTimeout, CheckUnsupported, CheckSkipped:
		return true
	}
	return false
}

// CountsAsFailure は連続失敗カウンタに加算すべき結果かどうかを返す。
// unsupported と skipped はサービス障害と同一視しない(仕様8.6)。
func (s CheckStatus) CountsAsFailure() bool {
	return s == CheckFailure || s == CheckTimeout
}

// State は監視対象の現在状態(Monitor State)。
// 状態遷移の判定ロジックは Phase 6 でこのパッケージに一元化する。
type State string

const (
	StateUnknown     State = "unknown"
	StateOperational State = "operational"
	StateDegraded    State = "degraded"
	StateOutage      State = "outage"
)

// Valid は定義済みの値かどうかを返す。
func (s State) Valid() bool {
	switch s {
	case StateUnknown, StateOperational, StateDegraded, StateOutage:
		return true
	}
	return false
}
