package admin

import (
	"net/http"
	"net/url"
	"os"

	"github.com/gin-gonic/gin"
)

func proxyLabel(raw string) string {
	if raw == "" {
		return "Not configured"
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "Invalid proxy URL"
	}
	// Never send proxy authentication credentials to the browser.
	u.User = nil
	return u.String()
}

func proxyEnv(name string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return os.Getenv(map[string]string{"HTTP_PROXY": "http_proxy", "HTTPS_PROXY": "https_proxy", "NO_PROXY": "no_proxy"}[name])
}

func (a *admin) proxyPage(ctx *gin.Context) {
	a.renderProxyPage(ctx, http.StatusOK, "")
}

func (a *admin) renderProxyPage(ctx *gin.Context, status int, message string) {
	settings := a.srv.ProxySettings()
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.Header("Cache-Control", "no-store")
	ctx.Status(status)
	templates.ExecuteTemplate(ctx.Writer, "proxy", gin.H{ // nolint: errcheck
		"Settings": settings, "CustomProxy": proxyLabel(settings.URL),
		"HTTPProxy": proxyLabel(proxyEnv("HTTP_PROXY")), "HTTPSProxy": proxyLabel(proxyEnv("HTTPS_PROXY")),
		"NoProxy": proxyEnv("NO_PROXY"), "CSRFToken": a.csrfToken(ctx),
		"Error": message, "Saved": ctx.Query("saved") == "1",
	})
}

func (a *admin) proxyUpdate(ctx *gin.Context) {
	settings := a.srv.ProxySettings()
	settings.Enabled = ctx.PostForm("enabled") == "on"
	switch ctx.PostForm("source") {
	case "environment":
		settings.UseEnvironment = true
	case "custom":
		settings.UseEnvironment = false
	default:
		a.renderProxyPage(ctx, http.StatusBadRequest, "Choose an environment or custom proxy.")
		return
	}
	if value := ctx.PostForm("proxy_url"); value != "" {
		settings.URL = value
	}
	if err := a.srv.SetProxySettings(settings); err != nil {
		a.renderProxyPage(ctx, http.StatusBadRequest, "Could not save proxy settings. Enter a valid HTTP or SOCKS5 URL; leave it blank to keep a saved URL.")
		return
	}
	ctx.Redirect(http.StatusSeeOther, "/admin/settings/proxy?saved=1")
}
