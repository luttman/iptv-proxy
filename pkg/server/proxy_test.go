package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/config"
	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/store"
	xtreamapi "github.com/pierre-emmanuelJ/iptv-proxy/pkg/xtream-proxy"
)

func proxyTestServer(t *testing.T) *Config {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "proxy.db"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() }) // nolint: errcheck
	srv, err := NewServer(&config.ProxyConfig{HostConfig: &config.HostConfiguration{}}, st)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.httpClient.CloseIdleConnections)
	return srv
}

func TestCustomProxyRoutesAPIAndDisables(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "direct") }))
	defer origin.Close()
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !r.URL.IsAbs() {
			t.Error("request did not use HTTP proxy")
		}
		switch r.URL.Query().Get("action") {
		case "get_live_streams", "get_vod_streams":
			_, _ = io.WriteString(w, `[{"stream_id":1,"name":"Channel"}]`)
		case "get_live_categories":
			_, _ = io.WriteString(w, `[{"category_id":"1","category_name":"TV"}]`)
		default:
			if r.URL.Path == "/player_api.php" {
				_, _ = io.WriteString(w, `{"user_info":{"auth":1},"server_info":{}}`)
			} else {
				_, _ = io.WriteString(w, "proxied")
			}
		}
	}))
	defer proxy.Close()
	srv := proxyTestServer(t)
	settings := store.ProxySettings{Enabled: true, URL: proxy.URL}
	if err := srv.SetProxySettings(settings); err != nil {
		t.Fatal(err)
	}
	assertBody := func(want string) {
		t.Helper()
		resp, err := srv.httpClient.Get(origin.URL)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close() // nolint: errcheck
		if err != nil || string(body) != want {
			t.Fatalf("body = %q, err = %v, want %q", body, err, want)
		}
	}
	assertBody("proxied")
	cli, err := xtreamapi.NewWithHTTP(context.Background(), "user", "pass", "http://unresolvable.invalid", "test", srv.httpClient)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cli.GetLiveCategories(); err != nil {
		t.Fatal(err)
	}
	if streams, err := cli.GetLiveStreams("1"); err != nil || len(streams) != 1 {
		t.Fatalf("live streams: %v %v", streams, err)
	}
	if streams, err := cli.GetVideoOnDemandStreams("1"); err != nil || len(streams) != 1 {
		t.Fatalf("VOD streams: %v %v", streams, err)
	}
	settings.Enabled = false
	if err := srv.SetProxySettings(settings); err != nil {
		t.Fatal(err)
	}
	assertBody("direct")
	settings.Enabled = true
	if err := srv.SetProxySettings(settings); err != nil {
		t.Fatal(err)
	}
	assertBody("proxied")
	for _, raw := range []string{"ftp://host", "http://", "http://host:bad", "http://host:99999", "http://host/path", "http://host?token=secret"} {
		settings.URL = raw
		if err := srv.SetProxySettings(settings); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}

func TestProxyIgnoresEnvironment(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://proxy.example.com:3128")
	t.Setenv("HTTPS_PROXY", "http://proxy.example.com:3128")
	srv := proxyTestServer(t)
	if srv.ProxySettings().Enabled {
		t.Fatal("unconfigured proxy should be disabled")
	}
	req, _ := http.NewRequest(http.MethodGet, "https://provider.example.com", nil)
	if u, err := srv.outboundProxy.proxy(req); u != nil || err != nil {
		t.Fatalf("environment proxy used: %v %v", u, err)
	}
	if err := srv.SetProxySettings(store.ProxySettings{Enabled: true}); err == nil {
		t.Fatal("enabled an empty proxy")
	}
}

func TestOutboundProxyCheckDoesNotSave(t *testing.T) {
	status, body := http.StatusOK, `{"ip":"203.0.113.5"}`
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !r.URL.IsAbs() {
			t.Error("test did not use the proxy")
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	defer proxy.Close()
	srv := proxyTestServer(t)
	before := srv.ProxySettings()
	s := store.ProxySettings{URL: proxy.URL}
	message, err := testOutboundProxy(context.Background(), s, "http://proxy-check.invalid")
	if err != nil || !strings.Contains(message, "203.0.113.5") {
		t.Fatalf("test result: %s %v", message, err)
	}
	if srv.ProxySettings() != before {
		t.Fatal("test changed proxy settings")
	}
	status = http.StatusProxyAuthRequired
	if _, err := testOutboundProxy(context.Background(), s, "http://proxy-check.invalid"); err == nil || !strings.Contains(err.Error(), "authentication") {
		t.Fatalf("authentication result: %v", err)
	}
	status, body = http.StatusOK, "invalid"
	if _, err := testOutboundProxy(context.Background(), s, "http://proxy-check.invalid"); err == nil {
		t.Fatal("invalid result accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := testOutboundProxy(ctx, s, "http://proxy-check.invalid"); err == nil {
		t.Fatal("canceled test succeeded")
	}
	s.URL = "http://user:private-secret@host:bad"
	if _, err := testOutboundProxy(context.Background(), s, "http://proxy-check.invalid"); err == nil || strings.Contains(err.Error(), "private-secret") {
		t.Fatalf("unsafe error: %v", err)
	}
}
