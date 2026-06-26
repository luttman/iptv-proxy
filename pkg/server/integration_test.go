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

package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/config"
	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/store"
)

// newTestServer builds a real *Config (with its Gin router wired up
// exactly as NewServer does) backed by a fresh on-disk SQLite store.
func newTestServer(t *testing.T) *Config {
	t.Helper()

	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open() error: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	c, err := NewServer(&config.ProxyConfig{
		HostConfig:         &config.HostConfiguration{Hostname: "proxy.example.com", Port: 8080},
		AdvertisedPort:     8080,
		M3UCacheExpiration: 1,
	}, st)
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}

	return c
}

func TestLiveStream_RoutesToAssignedBackend(t *testing.T) {
	var hitPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitPath = r.URL.Path
		w.Write([]byte("stream-bytes")) // nolint: errcheck
	}))
	defer upstream.Close()

	c := newTestServer(t)
	proxy := httptest.NewServer(c.Router)
	defer proxy.Close()

	xc, err := c.Store.CreateXtreamCode("provider-a", upstream.URL, "xuser", "xpass")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}
	if _, err := c.Store.CreateUser("alice", "hunter2", xc.ID); err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}

	resp, err := http.Get(proxy.URL + "/live/alice/hunter2/42.ts")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", resp.StatusCode, http.StatusOK, body)
	}
	if string(body) != "stream-bytes" {
		t.Errorf("body = %q, want %q", body, "stream-bytes")
	}

	wantPath := "/live/xuser/xpass/42.ts"
	if hitPath != wantPath {
		t.Errorf("upstream received path %q, want %q", hitPath, wantPath)
	}
}

func TestLiveStream_WrongPasswordRejected(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream should not have been contacted for an invalid login")
	}))
	defer upstream.Close()

	c := newTestServer(t)
	proxy := httptest.NewServer(c.Router)
	defer proxy.Close()

	xc, err := c.Store.CreateXtreamCode("provider-a", upstream.URL, "xuser", "xpass")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}
	if _, err := c.Store.CreateUser("alice", "hunter2", xc.ID); err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}

	resp, err := http.Get(proxy.URL + "/live/alice/wrong-password/42.ts")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestLiveStream_TwoUsersDifferentBackends(t *testing.T) {
	var hitsA, hitsB int
	upstreamA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hitsA++ }))
	defer upstreamA.Close()
	upstreamB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hitsB++ }))
	defer upstreamB.Close()

	c := newTestServer(t)
	proxy := httptest.NewServer(c.Router)
	defer proxy.Close()

	xcA, err := c.Store.CreateXtreamCode("provider-a", upstreamA.URL, "xuserA", "xpassA")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}
	xcB, err := c.Store.CreateXtreamCode("provider-b", upstreamB.URL, "xuserB", "xpassB")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}

	if _, err := c.Store.CreateUser("alice", "alicepass", xcA.ID); err != nil {
		t.Fatalf("CreateUser(alice) error: %v", err)
	}
	if _, err := c.Store.CreateUser("bob", "bobpass", xcB.ID); err != nil {
		t.Fatalf("CreateUser(bob) error: %v", err)
	}

	for _, path := range []string{"/live/alice/alicepass/1.ts", "/live/bob/bobpass/1.ts"} {
		resp, err := http.Get(proxy.URL + path)
		if err != nil {
			t.Fatalf("GET %q error: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("request %q: status = %d, want 200", path, resp.StatusCode)
		}
	}

	if hitsA != 1 {
		t.Errorf("provider A hits = %d, want 1", hitsA)
	}
	if hitsB != 1 {
		t.Errorf("provider B hits = %d, want 1", hitsB)
	}
}
