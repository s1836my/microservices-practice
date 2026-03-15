package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yourorg/micromart/services/gateway/internal/middleware"
)

func setupRateLimitRouter(rps float64, burst int) *gin.Engine {
	r := gin.New()
	r.Use(middleware.RateLimit(rps, burst))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	return r
}

func TestRateLimit_WithinBurst_Passes(t *testing.T) {
	router := setupRateLimitRouter(100, 5)

	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "request %d should pass", i)
	}
}

func TestRateLimit_ExceedsBurst_Returns429(t *testing.T) {
	// Very low rate (nearly zero) with burst of 2 so the 3rd request is blocked
	router := setupRateLimitRouter(0.001, 2)

	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "10.0.0.2:12345"
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "10.0.0.2:12345"
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "rate limit exceeded", body["error"])
	assert.Equal(t, "RATE_LIMIT_EXCEEDED", body["code"])
}

func TestRateLimit_DifferentIPs_IndependentLimits(t *testing.T) {
	router := setupRateLimitRouter(0.001, 1)

	// First IP: use up the burst
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "10.0.0.3:12345"
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// First IP: should be rate limited
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "10.0.0.3:12345"
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusTooManyRequests, w2.Code)

	// Second IP: should still be allowed
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req3.RemoteAddr = "10.0.0.4:12345"
	router.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
}
