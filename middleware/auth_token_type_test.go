package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const testPOSKey = "test-pos-jwt-key-0123456789abcdef0123456789"

func runAuth(t *testing.T, token string) (int, map[string]interface{}) {
	t.Helper()
	t.Setenv("JWT_KEY", testPOSKey)
	gin.SetMode(gin.TestMode)

	seen := map[string]interface{}{}
	r := gin.New()
	r.GET("/p", AuthMiddleware(), func(c *gin.Context) {
		for _, k := range []string{"user_id", "actor_type"} {
			if v, ok := c.Get(k); ok {
				seen[k] = v
			}
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/p", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec.Code, seen
}

func sign(t *testing.T, key string, claims jwt.MapClaims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(key))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func exp() int64 { return time.Now().Add(time.Hour).Unix() }

func TestAuthAcceptsCashierTokenFromGymAPI(t *testing.T) {
	code, seen := runAuth(t, sign(t, testPOSKey, jwt.MapClaims{
		"karyawan_id": 15, "typ": "karyawan_app", "userid": 15, "actor_type": "karyawan", "exp": exp(),
	}))
	if code != http.StatusOK {
		t.Fatalf("cashier token: want 200, got %d", code)
	}
	if seen["user_id"] != int64(15) || seen["actor_type"] != "karyawan" {
		t.Fatalf("cashier identity not set: %+v", seen)
	}
}

func TestAuthAcceptsStaffToken(t *testing.T) {
	code, seen := runAuth(t, sign(t, testPOSKey, jwt.MapClaims{
		"typ": "staff", "userid": 3, "actor_type": "outlet", "role_code": 1, "exp": exp(),
	}))
	if code != http.StatusOK || seen["actor_type"] != "outlet" || seen["user_id"] != int64(3) {
		t.Fatalf("staff token: got %d %+v", code, seen)
	}
}

func TestAuthRejectsNonPOSTokens(t *testing.T) {
	cases := map[string]string{
		"member token":        sign(t, testPOSKey, jwt.MapClaims{"member_id": 42, "typ": "member", "exp": exp()}),
		"pt token":            sign(t, testPOSKey, jwt.MapClaims{"pt_user_id": 7, "typ": "pt", "exp": exp()}),
		"untyped legacy":      sign(t, testPOSKey, jwt.MapClaims{"member_id": 42, "exp": exp()}),
		"no expiry":           sign(t, testPOSKey, jwt.MapClaims{"karyawan_id": 15, "typ": "karyawan_app"}),
		"expired":             sign(t, testPOSKey, jwt.MapClaims{"karyawan_id": 15, "typ": "karyawan_app", "exp": time.Now().Add(-time.Minute).Unix()}),
		"forged key":          sign(t, "attacker-key-attacker-key-attacker-key", jwt.MapClaims{"karyawan_id": 15, "typ": "karyawan_app", "exp": exp()}),
		"alg none (unsigned)": "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJ0eXAiOiJzdGFmZiJ9.",
	}
	for name, token := range cases {
		if code, _ := runAuth(t, token); code != http.StatusUnauthorized {
			t.Errorf("%s: want 401, got %d", name, code)
		}
	}
}
