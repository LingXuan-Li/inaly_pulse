// Package uptime は稼働率(Availability / Health / Coverage)とレイテンシ統計の
// 計算ロジックを置く。実装は Phase 9 で行う。
// Maintenance 期間の除外、unknown / unsupported の別集計、未実行をdowntimeにしない
// 扱いはこのパッケージで一元管理する(仕様13章)。
package uptime
