package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yourorg/micromart/services/gateway/internal/middleware"
)

const testSecret = "test-secret-key-for-unit-tests"

func init() {
	gin.SetMode(gin.TestMode)
}

func setupAuthRouter() *gin.Engine {
	r := gin.New()
	r.Use(middleware.Auth(testSecret))
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_id":    c.GetString("user_id"),
			"user_email": c.GetString("user_email"),
			"user_role":  c.GetString("user_role"),
		})
	})
	return r
}

func issueToken(userID, email, role string, expiresAt time.Time, secret string) string {
	claims := middleware.UserClaims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "micromart-user-service",
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(secret))
	return signed
}

func TestAuth_NoAuthorizationHeader(t *testing.T) {
	router := setupAuthRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "authorization header required", body["error"])
	assert.Equal(t, "UNAUTHENTICATED", body["code"])
}

func TestAuth_InvalidFormat_NoBearerPrefix(t *testing.T) {
	router := setupAuthRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Token some-token-value")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "invalid authorization header format", body["error"])
}

func TestAuth_InvalidFormat_OnlyBearer(t *testing.T) {
	router := setupAuthRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_ExpiredToken(t *testing.T) {
	expired := issueToken("user-1", "test@example.com", "customer", time.Now().Add(-1*time.Hour), testSecret)
	router := setupAuthRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+expired)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "invalid or expired token", body["error"])
}

func TestAuth_InvalidSignature(t *testing.T) {
	wrongSecret := issueToken("user-1", "test@example.com", "customer", time.Now().Add(1*time.Hour), "wrong-secret")
	router := setupAuthRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+wrongSecret)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "invalid or expired token", body["error"])
}

func TestAuth_WrongSigningMethod(t *testing.T) {
	// Create a token signed with "none" method (unsigned)
	claims := middleware.UserClaims{
		UserID: "user-1",
		Email:  "test@example.com",
		Role:   "customer",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "micromart-user-service",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	signed, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)

	router := setupAuthRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+signed)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_WrongIssuer(t *testing.T) {
	claims := middleware.UserClaims{
		UserID: "user-1",
		Email:  "test@example.com",
		Role:   "customer",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "wrong-issuer",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(testSecret))

	router := setupAuthRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+signed)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_ValidToken_SetsContext(t *testing.T) {
	validToken := issueToken("user-42", "alice@example.com", "seller", time.Now().Add(1*time.Hour), testSecret)
	router := setupAuthRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+validToken)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "user-42", body["user_id"])
	assert.Equal(t, "alice@example.com", body["user_email"])
	assert.Equal(t, "seller", body["user_role"])
}

func TestAuth_BearerCaseInsensitive(t *testing.T) {
	validToken := issueToken("user-1", "test@example.com", "customer", time.Now().Add(1*time.Hour), testSecret)
	router := setupAuthRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "BEARER "+validToken)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuth_MalformedJWT(t *testing.T) {
	router := setupAuthRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer not.a.valid.jwt.token")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
