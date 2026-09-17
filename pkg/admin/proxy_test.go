package admin

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/config"
	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/server"
)

func TestProxyTestExpiredSessionReturnsJSON(t *testing.T) {
	_, ts := newTestAdminServer(t)
	for _, cookie := range []string{"", "expired-session"} {
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/admin/settings/proxy/test", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Accept", "application/json")
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: cookie})
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			Message string `json:"message"`
		}
		err = json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close() // nolint: errcheck
		if resp.StatusCode != http.StatusUnauthorized || err != nil || !strings.Contains(result.Message, "Session expired") {
			t.Fatalf("unauthenticated test: status=%d message=%q err=%v", resp.StatusCode, result.Message, err)
		}
		if resp.Request.URL.Path != "/admin/settings/proxy/test" {
			t.Fatal("API request redirected to HTML login page")
		}
	}
}

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
	resp, err = client.PostForm(ts.URL+"/admin/settings/proxy/test", url.Values{"source": {"custom"}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close() // nolint: errcheck
	if resp.StatusCode != http.StatusForbidden {
		t.Fatal("proxy test requires CSRF")
	}
	before := srv.ProxySettings()
	resp, err = client.PostForm(ts.URL+"/admin/settings/proxy/test", url.Values{"source": {"custom"}, "csrf_token": {token}, "proxy_url": {"http://host:bad"}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close() // nolint: errcheck
	if resp.StatusCode != http.StatusBadGateway || srv.ProxySettings() != before {
		t.Fatal("failed proxy test changed settings or returned success")
	}
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
	if resp.StatusCode != http.StatusOK || resp.Request.URL.Path != "/admin" || !strings.Contains(string(body), `id="proxyDialog"`) {
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
