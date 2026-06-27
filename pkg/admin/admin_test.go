/*
 * Iptv-Proxy is a project to proxyfie an m3u file and to proxyfie an Xtream iptv service (client API).
 * Copyright (C) 2020  Pierre-Emmanuel Jacquier
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package admin

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/config"
	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/server"
	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/store"
)

func newTestAdminServer(t *testing.T) (*server.Config, *httptest.Server) {
	t.Helper()

	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open() error: %v", err)
	}
	t.Cleanup(func() { st.Close() }) // nolint: errcheck

	srv, err := server.NewServer(&config.ProxyConfig{
		HostConfig: &config.HostConfiguration{Hostname: "proxy.example.com", Port: 8080},
	}, st)
	if err != nil {
		t.Fatalf("server.NewServer() error: %v", err)
	}

	if err := Register(srv, Credentials{Username: "admin", Password: "adminpass"}); err != nil {
		t.Fatalf("Register() error: %v", err)
	}

	ts := httptest.NewServer(srv.Router)
	t.Cleanup(ts.Close)

	return srv, ts
}

// login posts valid admin credentials and returns the resulting
// cookie jar contents (session + csrf) for use by subsequent
// requests in the test.
func login(t *testing.T, client *http.Client, baseURL string) {
	t.Helper()

	resp, err := client.PostForm(baseURL+"/admin/login", url.Values{
		"username": {"admin"},
		"password": {"adminpass"},
	})
	if err != nil {
		t.Fatalf("login POST error: %v", err)
	}
	defer resp.Body.Close() // nolint: errcheck

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login: status = %d, want 200 (gin redirect followed by client)", resp.StatusCode)
	}
}

func cookieValue(t *testing.T, jar http.CookieJar, rawURL, name string) string {
	t.Helper()

	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("url.Parse() error: %v", err)
	}

	for _, c := range jar.Cookies(u) {
		if c.Name == name {
			return c.Value
		}
	}

	t.Fatalf("cookie %q not found", name)
	return ""
}

func TestLogin_WrongCredentialsRejected(t *testing.T) {
	_, ts := newTestAdminServer(t)

	resp, err := http.PostForm(ts.URL+"/admin/login", url.Values{
		"username": {"admin"},
		"password": {"wrong"},
	})
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	defer resp.Body.Close() // nolint: errcheck

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestLogin_SetsSameSiteCookies(t *testing.T) {
	_, ts := newTestAdminServer(t)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}

	resp, err := client.PostForm(ts.URL+"/admin/login", url.Values{
		"username": {"admin"},
		"password": {"adminpass"},
	})
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	defer resp.Body.Close() // nolint: errcheck

	found := map[string]bool{}
	for _, c := range resp.Cookies() {
		if c.SameSite != http.SameSiteLaxMode {
			t.Errorf("cookie %q SameSite = %v, want Lax", c.Name, c.SameSite)
		}
		found[c.Name] = true
	}

	if !found[sessionCookieName] || !found[csrfCookieName] {
		t.Errorf("expected both %q and %q cookies, got %v", sessionCookieName, csrfCookieName, found)
	}
}

func TestDashboard_RequiresSession(t *testing.T) {
	_, ts := newTestAdminServer(t)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}

	resp, err := client.Get(ts.URL + "/admin")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close() // nolint: errcheck

	if resp.StatusCode != http.StatusFound {
		t.Errorf("status = %d, want %d (redirect to login)", resp.StatusCode, http.StatusFound)
	}
	if loc := resp.Header.Get("Location"); loc != "/admin/login" {
		t.Errorf("redirect target = %q, want %q", loc, "/admin/login")
	}
}

func TestCSRF_RejectsRequestWithoutToken(t *testing.T) {
	_, ts := newTestAdminServer(t)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}
	client := &http.Client{Jar: jar}
	login(t, client, ts.URL)

	// Deliberately omit csrf_token.
	resp, err := client.PostForm(ts.URL+"/admin/xtream-codes/new", url.Values{
		"name":            {"provider-a"},
		"base_url":        {"http://example.tv:8080"},
		"xtream_user":     {"xu"},
		"xtream_password": {"xp"},
	})
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	defer resp.Body.Close() // nolint: errcheck

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestCSRF_AcceptsRequestWithValidToken(t *testing.T) {
	srv, ts := newTestAdminServer(t)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}
	client := &http.Client{Jar: jar}
	login(t, client, ts.URL)

	token := cookieValue(t, jar, ts.URL+"/admin", csrfCookieName)

	resp, err := client.PostForm(ts.URL+"/admin/xtream-codes/new", url.Values{
		"name":            {"provider-a"},
		"base_url":        {"http://example.tv:8080"},
		"xtream_user":     {"xu"},
		"xtream_password": {"xp"},
		"csrf_token":      {token},
	})
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	defer resp.Body.Close() // nolint: errcheck

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (redirect followed)", resp.StatusCode)
	}

	codes, err := srv.Store.ListXtreamCodes()
	if err != nil {
		t.Fatalf("ListXtreamCodes() error: %v", err)
	}
	if len(codes) != 1 || codes[0].Name != "provider-a" {
		t.Errorf("ListXtreamCodes() = %+v, want one entry named provider-a", codes)
	}
}

func TestCSRF_RejectsMismatchedToken(t *testing.T) {
	_, ts := newTestAdminServer(t)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}
	client := &http.Client{Jar: jar}
	login(t, client, ts.URL)

	resp, err := client.PostForm(ts.URL+"/admin/xtream-codes/new", url.Values{
		"name":            {"provider-a"},
		"base_url":        {"http://example.tv:8080"},
		"xtream_user":     {"xu"},
		"xtream_password": {"xp"},
		"csrf_token":      {"not-the-real-token"},
	})
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	defer resp.Body.Close() // nolint: errcheck

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}
