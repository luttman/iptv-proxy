package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/store"
)

func (a *admin) proxyPage(ctx *gin.Context) {
	ctx.Redirect(http.StatusFound, "/admin")
}

func (a *admin) proxyTemplateData(ctx *gin.Context) gin.H {
	return gin.H{"Settings": a.srv.ProxySettings(), "CSRFToken": a.csrfToken(ctx)}
}

func (a *admin) proxySettingsFromForm(ctx *gin.Context) store.ProxySettings {
	settings := a.srv.ProxySettings()
	settings.Enabled = ctx.PostForm("enabled") == "on"
	if value := ctx.PostForm("proxy_url"); value != "" {
		settings.URL = value
	}
	return settings
}

func (a *admin) proxyUpdate(ctx *gin.Context) {
	settings := a.proxySettingsFromForm(ctx)
	if err := a.srv.SetProxySettings(settings); err != nil {
		ctx.String(http.StatusBadRequest, "Could not save proxy settings. Enter a valid HTTP or SOCKS5 URL; leave it blank to keep a saved URL.")
		return
	}
	ctx.Redirect(http.StatusSeeOther, "/admin")
}

func (a *admin) proxyTest(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	settings := a.proxySettingsFromForm(ctx)
	message, err := a.srv.TestOutboundProxy(ctx.Request.Context(), settings)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": message})
}
