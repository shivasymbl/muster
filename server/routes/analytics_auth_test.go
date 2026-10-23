package routes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/shivasymbl/muster/server/logger"
)

func TestAnalyticsBasicAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.Init(io.Discard)
	r := gin.New()
	r.GET("/x", AnalyticsBasicAuth(), func(c *gin.Context) { c.Status(http.StatusOK) })

	do := func(user, pass string) int {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.SetBasicAuth(user, pass)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	t.Setenv("ANALYTICS_USERNAME", "")
	t.Setenv("ANALYTICS_PASSWORD", "")
	if got := do("", ""); got != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured + empty creds: got %d, want 503", got)
	}

	t.Setenv("ANALYTICS_USERNAME", "admin")
	t.Setenv("ANALYTICS_PASSWORD", "s3cret")
	if got := do("admin", "wrong"); got != http.StatusUnauthorized {
		t.Fatalf("wrong password: got %d, want 401", got)
	}
	if got := do("admin", "s3cret"); got != http.StatusOK {
		t.Fatalf("valid creds: got %d, want 200", got)
	}
}
