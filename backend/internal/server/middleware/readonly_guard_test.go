package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestReadOnlyGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)

	run := func(role, method string) int {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			if role != "" {
				c.Set(string(ContextKeyUserRole), role)
			}
			c.Next()
		})
		router.Use(ReadOnlyGuard())
		handle := func(c *gin.Context) { c.Status(http.StatusOK) }
		router.Handle(method, "/x", handle)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/x", nil)
		router.ServeHTTP(w, req)
		return w.Code
	}

	runPath := func(role, method, target string) int {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			if role != "" {
				c.Set(string(ContextKeyUserRole), role)
			}
			c.Next()
		})
		router.Use(ReadOnlyGuard())
		router.Handle(method, target, func(c *gin.Context) { c.Status(http.StatusOK) })
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(method, target, nil))
		return w.Code
	}

	t.Run("readonly blocks writes", func(t *testing.T) {
		for _, method := range []string{"POST", "PUT", "DELETE", "PATCH"} {
			require.Equal(t, http.StatusForbidden, run(service.RoleReadOnly, method), method)
		}
	})
	t.Run("readonly allows reads", func(t *testing.T) {
		require.Equal(t, http.StatusOK, run(service.RoleReadOnly, "GET"))
		require.Equal(t, http.StatusOK, run(service.RoleReadOnly, "HEAD"))
		require.Equal(t, http.StatusOK, run(service.RoleReadOnly, "OPTIONS"))
	})
	t.Run("admin unaffected", func(t *testing.T) {
		require.Equal(t, http.StatusOK, run(service.RoleAdmin, "POST"))
		require.Equal(t, http.StatusOK, run(service.RoleAdmin, "DELETE"))
	})
	t.Run("missing role rejected", func(t *testing.T) {
		require.Equal(t, http.StatusUnauthorized, run("", "GET"))
	})
	t.Run("readonly allows whitelisted access-control/query POSTs", func(t *testing.T) {
		for _, target := range []string{
			"/api/v1/admin/compliance/accept",
			"/api/v1/admin/dashboard/users-usage",
			"/api/v1/admin/dashboard/api-keys-usage",
		} {
			require.Equal(t, http.StatusOK, runPath(service.RoleReadOnly, "POST", target), target)
			// admin 不受影响
			require.Equal(t, http.StatusOK, runPath(service.RoleAdmin, "POST", target), target)
		}
	})
	t.Run("readonly still blocks non-whitelist POST", func(t *testing.T) {
		require.Equal(t, http.StatusForbidden, runPath(service.RoleReadOnly, "POST", "/api/v1/admin/users"))
		require.Equal(t, http.StatusForbidden, runPath(service.RoleReadOnly, "POST", "/api/v1/admin/compliance/accept/extra"))
	})
}
