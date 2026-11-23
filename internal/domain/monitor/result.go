package monitor

import "time"

// Details は監視方式ごとの固有の結果を表す。
// 共通の Result に方式固有の項目を詰め込まないため、各方式が自身の型で実装する。
type Details interface {
	Type() Type
}

// CheckError は監視失敗時のエラー情報。認証情報などの機密は含めない。
type CheckError struct {
	Kind    string // 例: "timeout", "dns_nxdomain", "connection_refused"
	Message string
}

func (e *CheckError) Error() string {
	if e == nil {
		return ""
	}
	return e.Kind + ": " + e.Message
}

// Result は1回の監視実行の結果(Check Result)。
type Result struct {
	MonitorID  string
	Type       Type
	Status     CheckStatus
	StartedAt  time.Time
	FinishedAt time.Time
	Duration   time.Duration
	Error      *CheckError
	Details    Details
}
