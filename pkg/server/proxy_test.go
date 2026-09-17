package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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

// ProxyFromEnvironment caches its configuration process-wide. A subprocess
// gives this check an isolated environment regardless of other HTTP tests.
func TestEnvironmentProxyToggle(t *testing.T) {
	if os.Getenv("IPTV_PROXY_ENV_TEST") == "1" {
		srv := proxyTestServer(t)
		req, _ := http.NewRequest(http.MethodGet, "http://provider.example.com", nil)
		u, err := srv.outboundProxy.proxy(req)
		if err != nil || u == nil || u.String() != "http://proxy.example.com:3128" {
			t.Fatalf("environment proxy: %v %v", u, err)
		}
		settings := srv.ProxySettings()
		settings.Enabled = false
		if err := srv.SetProxySettings(settings); err != nil {
			t.Fatal(err)
		}
		if u, err := srv.outboundProxy.proxy(req); u != nil || err != nil {
			t.Fatalf("disabled environment proxy: %v %v", u, err)
		}
		settings.Enabled = true
		if err := srv.SetProxySettings(settings); err != nil {
			t.Fatal(err)
		}
		if u, err := srv.outboundProxy.proxy(req); u == nil || err != nil {
			t.Fatalf("re-enabled environment proxy: %v %v", u, err)
		}
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestEnvironmentProxyToggle$")
	for _, env := range os.Environ() {
		name := strings.ToUpper(strings.SplitN(env, "=", 2)[0])
		if name != "HTTP_PROXY" && name != "HTTPS_PROXY" && name != "NO_PROXY" && name != "REQUEST_METHOD" && name != "IPTV_PROXY_ENV_TEST" {
			cmd.Env = append(cmd.Env, env)
		}
	}
	cmd.Env = append(cmd.Env, "IPTV_PROXY_ENV_TEST=1", "HTTP_PROXY=http://proxy.example.com:3128", "NO_PROXY=")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("environment check: %v\n%s", err, output)
	}
}
