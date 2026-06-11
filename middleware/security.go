package middleware

import (
	"bytes"
	"encoding/json"
	"html"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func bypassXSSSanitizer(c *gin.Context) bool {
	if strings.EqualFold(strings.TrimSpace(c.GetHeader("Upgrade")), "websocket") {
		return true
	}
	return strings.HasPrefix(c.Request.URL.Path, "/datauploads/")
}

func unsafeJSONText(value string) bool {
	lower := strings.ToLower(value)
	return strings.ContainsAny(value, "<>") ||
		strings.Contains(lower, "javascript:") ||
		strings.Contains(lower, "data:text/html")
}

func sanitizeJSONValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case string:
		if unsafeJSONText(typed) {
			return html.EscapeString(typed)
		}
		return typed
	case []interface{}:
		for i := range typed {
			typed[i] = sanitizeJSONValue(typed[i])
		}
		return typed
	case map[string]interface{}:
		for key, item := range typed {
			typed[key] = sanitizeJSONValue(item)
		}
		return typed
	default:
		return value
	}
}

func looksLikeJSON(body []byte) bool {
	trimmed := bytes.TrimSpace(body)
	return len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[')
}

func sanitizeJSONBytes(body []byte) ([]byte, bool) {
	if !looksLikeJSON(body) {
		return body, false
	}

	var decoded interface{}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return body, false
	}
	if _, err := decoder.Token(); err != io.EOF {
		return body, false
	}

	encoded, err := json.Marshal(sanitizeJSONValue(decoded))
	if err != nil {
		return body, false
	}
	return encoded, true
}

func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		headers := c.Writer.Header()
		headers.Set("X-Content-Type-Options", "nosniff")
		headers.Set("X-Frame-Options", "DENY")
		headers.Set("Referrer-Policy", "no-referrer")
		headers.Set("Cross-Origin-Opener-Policy", "same-origin")
		if strings.HasPrefix(c.Request.URL.Path, "/datauploads/") {
			headers.Set("Cross-Origin-Resource-Policy", "cross-origin")
		} else {
			headers.Set("Cross-Origin-Resource-Policy", "same-origin")
		}
		headers.Set("Permissions-Policy", "camera=(), geolocation=(), microphone=()")
		headers.Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; script-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; connect-src 'self' https: http: ws: wss:")
		c.Next()
	}
}

func SanitizeJSONRequests() gin.HandlerFunc {
	return func(c *gin.Context) {
		if bypassXSSSanitizer(c) || c.Request.Body == nil {
			c.Next()
			return
		}

		contentType := strings.ToLower(c.GetHeader("Content-Type"))
		if !strings.Contains(contentType, "application/json") {
			c.Next()
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
		_ = c.Request.Body.Close()

		if sanitized, ok := sanitizeJSONBytes(body); ok {
			body = sanitized
			c.Request.ContentLength = int64(len(body))
			c.Request.Header.Set("Content-Length", strconv.Itoa(len(body)))
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Next()
	}
}

type sanitizingResponseWriter struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func (w *sanitizingResponseWriter) Write(data []byte) (int, error) {
	return w.body.Write(data)
}

func (w *sanitizingResponseWriter) WriteString(data string) (int, error) {
	return w.body.WriteString(data)
}

func SanitizeJSONResponses() gin.HandlerFunc {
	return func(c *gin.Context) {
		if bypassXSSSanitizer(c) {
			c.Next()
			return
		}

		writer := &sanitizingResponseWriter{ResponseWriter: c.Writer}
		c.Writer = writer
		c.Next()

		body := writer.body.Bytes()
		contentType := strings.ToLower(writer.Header().Get("Content-Type"))
		shouldSanitize := strings.Contains(contentType, "application/json") || looksLikeJSON(body)
		if shouldSanitize && len(body) > 0 {
			if sanitized, ok := sanitizeJSONBytes(body); ok {
				body = sanitized
				if !strings.Contains(contentType, "application/json") {
					writer.Header().Set("Content-Type", "application/json")
				}
			}
		}

		writer.ResponseWriter.WriteHeader(writer.Status())
		if len(body) > 0 {
			_, _ = writer.ResponseWriter.Write(body)
		}
	}
}
