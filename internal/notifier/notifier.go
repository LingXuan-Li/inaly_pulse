// Package notifier は通知先(Discord など)の共通インターフェースを定義する。
// 監視エンジンから独立しており、Slack やメールを追加する場合も Notifier を実装するだけでよい。
package notifier

import (
	"context"
	"time"
)

// EventKind は通知対象のイベント種別。settings.json の notify_on と対応する。
type EventKind string

const (
	EventIncidentCreated    EventKind = "incident_created"
	EventIncidentUpdated    EventKind = "incident_updated"
	EventIncidentResolved   EventKind = "incident_resolved"
	EventMaintenanceStarted EventKind = "maintenance_started"
	EventMaintenanceEnded   EventKind = "maintenance_ended"
)

// Event は通知する出来事。通知先に依存しない形で表現する。
type Event struct {
	Kind        EventKind
	DedupeKey   string // 同じイベントの重複通知を防ぐためのキー
	MonitorID   string
	MonitorName string
	IncidentID  string
	Summary     string
	OccurredAt  time.Time
	ResolvedAt  *time.Time
	IssueURL    string
}

// Notifier は1つの通知先。
// Notify の失敗は呼び出し側で記録するだけとし、監視処理全体を失敗させない(仕様12.4)。
type Notifier interface {
	Name() string
	Notify(ctx context.Context, e Event) error
}
