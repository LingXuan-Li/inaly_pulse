// Package storage は永続ストレージの抽象を定義する。
// 実装は Git(internal/storage/git)とテスト用のメモリ(internal/storage/memory)。
// このインターフェースは Git / GitHub API の語彙を含まず、パスとバイト列だけを扱う。
// 型付きのデータ(状態・履歴・障害など)の読み書きは、この上の層で JSON に変換して行う。
package storage

import (
	"context"
	"errors"
)

var (
	// ErrNotFound は指定のパスにファイルが存在しない。
	ErrNotFound = errors.New("storage: not found")
	// ErrConflict は更新中に他の更新が先に反映された(Ref更新のSHA競合など)。
	// Repository 実装が内部で再試行するため、通常は呼び出し側に見えない。
	ErrConflict = errors.New("storage: concurrent update conflict")
	// ErrRetriesExhausted は再試行回数の上限に達しても更新できなかった。
	ErrRetriesExhausted = errors.New("storage: retries exhausted")
)

// Snapshot はある時点(Revision)の一貫した読み取りビュー。パスは "/" 区切りのリポジトリ相対パス。
type Snapshot interface {
	// Revision はこのビューの識別子(Git の場合は Commit SHA)。
	Revision() string
	// Read はファイル内容を返す。存在しなければ ErrNotFound。
	Read(ctx context.Context, path string) ([]byte, error)
	// List は prefix 配下のファイルパスを返す。
	List(ctx context.Context, prefix string) ([]string, error)
}

// ChangeSet は1回のコミットにまとめる変更。
type ChangeSet struct {
	Message string
	Writes  map[string][]byte // パス -> 新しい内容
	Deletes []string
}

// IsEmpty は変更が1つも無いかを返す。
// 実装は、Writes の内容が既存と同一の場合も「変更なし」として扱い、コミットを作らない。
func (c ChangeSet) IsEmpty() bool {
	return len(c.Writes) == 0 && len(c.Deletes) == 0
}

// UpdateFunc は最新の Snapshot から ChangeSet を作る関数。
// 競合時には最新状態で再実行されるため、Snapshot 以外の状態に依存せず、
// 何度実行しても同じ結果を返す(冪等な)関数でなければならない。
type UpdateFunc func(ctx context.Context, snap Snapshot) (ChangeSet, error)

// Result は Update の結果。
type Result struct {
	Committed bool   // false の場合は変更なしでコミットを作っていない
	Revision  string // 更新後(または変更なしの場合は現在)の Revision
	Attempts  int    // fn を実行した回数
}

// Repository は永続ストレージ。
type Repository interface {
	// Snapshot は最新の状態を読み取るビューを返す。
	Snapshot(ctx context.Context) (Snapshot, error)

	// Update は最新状態を読み、fn が返した変更を1回のコミットとして反映する。
	// 競合した場合は最新状態を再取得して fn を再実行する(回数は実装が制限する)。
	// 変更が無い場合はコミットを作らず Committed=false を返す。
	// 反映に成功していない限り、必ず error を返す(成功として扱わない)。
	Update(ctx context.Context, fn UpdateFunc) (Result, error)
}
