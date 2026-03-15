package health

import "context"

// Pinger は PingContext メソッドを持つインターフェース。
// database/sql の *sql.DB が暗黙的に実装している。
type Pinger interface {
	PingContext(ctx context.Context) error
}

// PingChecker は Pinger を使ってヘルスチェックを行う。
type PingChecker struct {
	name   string
	pinger Pinger
}

// NewPingChecker は PingChecker を生成する。
func NewPingChecker(name string, p Pinger) *PingChecker {
	return &PingChecker{name: name, pinger: p}
}

func (c *PingChecker) Name() string {
	return c.name
}

func (c *PingChecker) Check(ctx context.Context) error {
	return c.pinger.PingContext(ctx)
}

// CheckerFunc は任意の関数を Checker に変換するアダプタ。
// Redis など Pinger インターフェースを実装しない依存を扱う際に使う。
type CheckerFunc struct {
	name    string
	checkFn func(ctx context.Context) error
}

// NewCheckerFunc は CheckerFunc を生成する。
func NewCheckerFunc(name string, fn func(ctx context.Context) error) *CheckerFunc {
	return &CheckerFunc{name: name, checkFn: fn}
}

func (c *CheckerFunc) Name() string {
	return c.name
}

func (c *CheckerFunc) Check(ctx context.Context) error {
	return c.checkFn(ctx)
}
