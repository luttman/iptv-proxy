package admin

import (
	"errors"
	"net/http"
	"net/url"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/store"
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
	ctx.Redirect(http.StatusFound, "/admin")
}

func (a *admin) proxyTemplateData(ctx *gin.Context) gin.H {
	settings := a.srv.ProxySettings()
	return gin.H{
		"Settings": settings, "CustomProxy": proxyLabel(settings.URL),
		"HTTPProxy": proxyLabel(proxyEnv("HTTP_PROXY")), "HTTPSProxy": proxyLabel(proxyEnv("HTTPS_PROXY")),
		"NoProxy": proxyEnv("NO_PROXY"), "CSRFToken": a.csrfToken(ctx),
	}
}

func (a *admin) proxySettingsFromForm(ctx *gin.Context) (store.ProxySettings, error) {
	settings := a.srv.ProxySettings()
	settings.Enabled = ctx.PostForm("enabled") == "on"
	switch ctx.PostForm("source") {
	case "environment":
		settings.UseEnvironment = true
	case "custom":
		settings.UseEnvironment = false
	default:
		return settings, errors.New("choose an environment or custom proxy")
	}
	if value := ctx.PostForm("proxy_url"); value != "" {
		settings.URL = value
	}
	return settings, nil
}

func (a *admin) proxyUpdate(ctx *gin.Context) {
	settings, err := a.proxySettingsFromForm(ctx)
	if err != nil {
		ctx.String(http.StatusBadRequest, "%s", err)
		return
	}
	if err := a.srv.SetProxySettings(settings); err != nil {
		ctx.String(http.StatusBadRequest, "Could not save proxy settings. Enter a valid HTTP or SOCKS5 URL; leave it blank to keep a saved URL.")
		return
	}
	ctx.Redirect(http.StatusSeeOther, "/admin")
}

func (a *admin) proxyTest(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	settings, err := a.proxySettingsFromForm(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	message, err := a.srv.TestOutboundProxy(ctx.Request.Context(), settings)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": message})
}
