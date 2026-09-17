//go:build unit

package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type proxyURLService struct {
	service.AdminService
	proxy *service.Proxy
}

func (s *proxyURLService) GetProxy(_ context.Context, id int64) (*service.Proxy, error) {
	s.proxy.ID = id
	return s.proxy, nil
}

func TestBuildProxyURL(t *testing.T) {
	for _, tc := range []struct {
		name  string
		proxy service.Proxy
		want  string
	}{
		{name: "full credentials", proxy: service.Proxy{Protocol: "socks5", Host: "h.example", Port: 1080, Username: "u", Password: "p"}, want: "socks5://u:p@h.example:1080"},
		{name: "username only", proxy: service.Proxy{Protocol: "http", Host: "h.example", Port: 8080, Username: "u"}, want: "http://u@h.example:8080"},
		{name: "password only", proxy: service.Proxy{Protocol: "https", Host: "h.example", Port: 8443, Password: "p"}, want: "https://:p@h.example:8443"},
		{name: "no auth", proxy: service.Proxy{Protocol: "http", Host: "h.example", Port: 3128}, want: "http://h.example:3128"},
		{name: "escapes credentials", proxy: service.Proxy{Protocol: "http", Host: "h.example", Port: 3128, Username: "u@x", Password: "p:/ q"}, want: "http://u%40x:p%3A%2F%20q@h.example:3128"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, BuildProxyURL(&tc.proxy))
		})
	}
}

func TestProxyHandlerGetProxyURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &proxyURLService{proxy: &service.Proxy{Protocol: "socks5", Host: "h.example", Port: 1080, Username: "u", Password: "secret"}}
	router := gin.New()
	router.GET("/proxies/:id/url", NewProxyHandler(svc).GetProxyURL)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/proxies/7/url", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var body struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "socks5://u:secret@h.example:1080", body.Data.URL)
}
