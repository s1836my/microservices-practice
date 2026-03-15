package health

import (
	"context"
	"encoding/json"
	"net/http"
)

// Status はヘルスチェック結果を表す。
type Status string

const (
	StatusOK       Status = "ok"
	StatusDegraded Status = "degraded"
)

// Checker はヘルスチェック対象のインターフェース。
type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

// Response はヘルスチェックの HTTP レスポンス。
type Response struct {
	Status  Status            `json:"status"`
	Checks  map[string]string `json:"checks,omitempty"`
	Version string            `json:"version,omitempty"`
}

// Handler は Readiness Probe 用 HTTP ハンドラーを返す。
// いずれかの Checker が失敗したとき HTTP 503 を返す。
func Handler(version string, checks ...Checker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := Response{
			Status:  StatusOK,
			Checks:  make(map[string]string),
			Version: version,
		}

		for _, c := range checks {
			if err := c.Check(r.Context()); err != nil {
				resp.Status = StatusDegraded
				resp.Checks[c.Name()] = err.Error()
			} else {
				resp.Checks[c.Name()] = "ok"
			}
		}

		code := http.StatusOK
		if resp.Status == StatusDegraded {
			code = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}
}

// LivenessHandler は kubelet の Liveness Probe 用ハンドラー。常に 200 を返す。
func LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"}) //nolint:errcheck
	}
}

// NewMux はヘルスチェック用 *http.ServeMux を返す。
//
//	GET /health -> Readiness (全 Checker を実行)
//	GET /ready  -> Readiness (全 Checker を実行)
//	GET /livez  -> Liveness (常に 200)
func NewMux(version string, checks ...Checker) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", Handler(version, checks...))
	mux.HandleFunc("GET /ready", Handler(version, checks...))
	mux.HandleFunc("GET /livez", LivenessHandler())
	return mux
}
