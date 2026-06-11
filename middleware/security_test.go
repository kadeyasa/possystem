package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGinSanitizeJSONRequestsEscapesIncomingPayloads(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SanitizeJSONRequests())
	router.POST("/api/products", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			t.Fatalf("failed reading request body: %v", err)
		}
		c.Data(http.StatusOK, "application/json", body)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/products", strings.NewReader(`{"name":"<img src=x onerror=alert()>"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var decoded struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("expected valid JSON, got %v", err)
	}
	if strings.Contains(decoded.Name, "<img") || !strings.Contains(decoded.Name, "&lt;img") {
		t.Fatalf("expected request payload to be escaped, got %q", decoded.Name)
	}
}

func TestGinSanitizeJSONResponsesEscapesStoredPayloads(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SanitizeJSONResponses())
	router.GET("/api/products", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"name": `"><img src=x onerror=alert()>`})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/products", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "<img") {
		t.Fatalf("expected response payload to be escaped, got %s", body)
	}

	var decoded struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("expected valid JSON, got %v", err)
	}
	if !strings.Contains(decoded.Name, "&lt;img") {
		t.Fatalf("expected escaped HTML entity, got %q", decoded.Name)
	}
}
