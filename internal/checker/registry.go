package checker

import (
	"fmt"
	"sort"

	"inaly_pulse/internal/domain/monitor"
)

// Registry は監視方式ごとの Checker を保持する。
// 新しい方式の追加は Register を呼ぶだけで済み、他の処理を変更しない。
type Registry struct {
	checkers map[monitor.Type]Checker
}

// NewRegistry は空の Registry を返す。
func NewRegistry() *Registry {
	return &Registry{checkers: make(map[monitor.Type]Checker)}
}

// Register は Checker を登録する。同じ Type の重複登録はエラーにする。
func (r *Registry) Register(c Checker) error {
	if c == nil {
		return fmt.Errorf("checker: nil checker")
	}
	t := c.Type()
	if t == "" {
		return fmt.Errorf("checker: empty type")
	}
	if _, exists := r.checkers[t]; exists {
		return fmt.Errorf("checker: type %q already registered", t)
	}
	r.checkers[t] = c
	return nil
}

// Get は Type に対応する Checker を返す。
func (r *Registry) Get(t monitor.Type) (Checker, bool) {
	c, ok := r.checkers[t]
	return c, ok
}

// Types は登録済みの Type を昇順で返す。設定検証で「登録済みか」を判定するために使う。
func (r *Registry) Types() []monitor.Type {
	types := make([]monitor.Type, 0, len(r.checkers))
	for t := range r.checkers {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool { return types[i] < types[j] })
	return types
}
