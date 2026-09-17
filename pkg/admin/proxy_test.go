package admin

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/config"
	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/server"
)

func TestProxySettingsSaveAndAccess(t *testing.T) {
	srv, ts := newTestAdminServer(t)
	resp, err := http.Get(ts.URL + "/admin/settings/proxy")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close() // nolint: errcheck
	if resp.Request.URL.Path != "/admin/login" {
		t.Fatal("settings require login")
	}
	client, token := newLoggedInClient(t, ts.URL)
	form := url.Values{"source": {"custom"}, "proxy_url": {"http://name:secret@proxy.example.com:3128"}, "enabled": {"on"}}
	resp, err = client.PostForm(ts.URL+"/admin/settings/proxy", form)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close() // nolint: errcheck
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("missing CSRF: %d", resp.StatusCode)
	}
	form.Set("csrf_token", token)
	resp, err = client.PostForm(ts.URL+"/admin/settings/proxy", form)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close() // nolint: errcheck
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "Proxy settings saved") {
		t.Fatalf("save: %d %s", resp.StatusCode, body)
	}
	if strings.Contains(string(body), "secret") || strings.Contains(string(body), "name@") {
		t.Fatal("proxy credentials exposed")
	}
	if !strings.Contains(string(body), "http://proxy.example.com:3128") {
		t.Fatal("saved proxy missing from UI")
	}
	restarted, err := server.NewServer(&config.ProxyConfig{HostConfig: &config.HostConfiguration{}}, srv.Store)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.ProxySettings() != srv.ProxySettings() {
		t.Fatal("settings not persisted")
	}
	form.Del("enabled")
	form.Set("source", "environment")
	form.Del("proxy_url")
	resp, err = client.PostForm(ts.URL+"/admin/settings/proxy", form)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close() // nolint: errcheck
	if srv.ProxySettings().Enabled || !srv.ProxySettings().UseEnvironment {
		t.Fatal("environment disable not saved")
	}
	form.Set("proxy_url", "http://host:bad")
	resp, err = client.PostForm(ts.URL+"/admin/settings/proxy", form)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close() // nolint: errcheck
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid URL: %d", resp.StatusCode)
	}
}
