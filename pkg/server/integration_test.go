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
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/config"
	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/store"
)

// newTestServer builds a real *Config (with its Gin router wired up
// exactly as NewServer does) backed by a fresh on-disk SQLite store.
func newTestServer(t *testing.T) *Config {
	t.Helper()

	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"), nil)
	if err != nil {
		t.Fatalf("store.Open() error: %v", err)
	}
	t.Cleanup(func() { st.Close() }) // nolint: errcheck

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
	if _, err := c.Store.CreateUser("alice", "hunter2", xc.ID, 0); err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}

	resp, err := http.Get(proxy.URL + "/live/alice/hunter2/42.ts")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close() // nolint: errcheck
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
	if _, err := c.Store.CreateUser("alice", "hunter2", xc.ID, 0); err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}

	resp, err := http.Get(proxy.URL + "/live/alice/wrong-password/42.ts")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close() // nolint: errcheck

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestLiveStream_FailsOverToBackupOnServerError(t *testing.T) {
	var badPath, goodPath string
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		badPath = r.URL.Path
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()

	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		goodPath = r.URL.Path
		w.Write([]byte("stream-bytes")) // nolint: errcheck
	}))
	defer good.Close()

	c := newTestServer(t)
	proxy := httptest.NewServer(c.Router)
	defer proxy.Close()

	xc, err := c.Store.CreateXtreamCode("provider-a", bad.URL+" "+good.URL, "xuser", "xpass")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}
	if _, err := c.Store.CreateUser("alice", "hunter2", xc.ID, 0); err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}

	// Seed monitoring data so both authentication and the in-request
	// failover deterministically prefer "bad" first, exercising the
	// retry path instead of racing which httptest server accepts the
	// bootstrap TCP probe fastest.
	c.upstreamHealth[bad.URL] = UpstreamHealth{Up: true, LatencyMS: 1}
	c.upstreamHealth[good.URL] = UpstreamHealth{Up: true, LatencyMS: 500}

	resp, err := http.Get(proxy.URL + "/live/alice/hunter2/42.ts")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close() // nolint: errcheck
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", resp.StatusCode, http.StatusOK, body)
	}
	if string(body) != "stream-bytes" {
		t.Errorf("body = %q, want %q", body, "stream-bytes")
	}
	if badPath != "/live/xuser/xpass/42.ts" || goodPath != "/live/xuser/xpass/42.ts" {
		t.Errorf("backup address did not serve the same account/channel path: bad=%q good=%q", badPath, goodPath)
	}
}

func TestLiveStream_DoesNotFailOverOnInvalidCredentials(t *testing.T) {
	var backupHit bool
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer primary.Close()

	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backupHit = true
		w.Write([]byte("stream-bytes")) // nolint: errcheck
	}))
	defer backup.Close()

	c := newTestServer(t)
	proxy := httptest.NewServer(c.Router)
	defer proxy.Close()

	xc, err := c.Store.CreateXtreamCode("provider-a", primary.URL+" "+backup.URL, "xuser", "xpass")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}
	if _, err := c.Store.CreateUser("alice", "hunter2", xc.ID, 0); err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}

	c.upstreamHealth[primary.URL] = UpstreamHealth{Up: true, LatencyMS: 1}
	c.upstreamHealth[backup.URL] = UpstreamHealth{Up: true, LatencyMS: 500}

	resp, err := http.Get(proxy.URL + "/live/alice/hunter2/42.ts")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close() // nolint: errcheck

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d (a 403 should not trigger failover)", resp.StatusCode, http.StatusForbidden)
	}
	if backupHit {
		t.Error("backup address was contacted for a 403 response; it shouldn't be")
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

	if _, err := c.Store.CreateUser("alice", "alicepass", xcA.ID, 0); err != nil {
		t.Fatalf("CreateUser(alice) error: %v", err)
	}
	if _, err := c.Store.CreateUser("bob", "bobpass", xcB.ID, 0); err != nil {
		t.Fatalf("CreateUser(bob) error: %v", err)
	}

	for _, path := range []string{"/live/alice/alicepass/1.ts", "/live/bob/bobpass/1.ts"} {
		resp, err := http.Get(proxy.URL + path)
		if err != nil {
			t.Fatalf("GET %q error: %v", path, err)
		}
		resp.Body.Close() // nolint: errcheck
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

func TestLiveStream_TrackedWhileActive(t *testing.T) {
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("chunk")) // nolint: errcheck
		w.(http.Flusher).Flush()
		<-release
	}))
	defer upstream.Close()

	c := newTestServer(t)
	proxy := httptest.NewServer(c.Router)
	defer proxy.Close()

	xc, err := c.Store.CreateXtreamCode("provider-a", upstream.URL, "xuser", "xpass")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}
	if _, err := c.Store.CreateUser("alice", "hunter2", xc.ID, 0); err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}

	if got := len(c.ActiveStreams()); got != 0 {
		t.Fatalf("ActiveStreams() before request = %d, want 0", got)
	}

	done := make(chan struct{})
	go func() {
		resp, err := http.Get(proxy.URL + "/live/alice/hunter2/42.ts")
		if err == nil {
			io.ReadAll(resp.Body) // nolint: errcheck
			resp.Body.Close()     // nolint: errcheck
		}
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if streams := c.ActiveStreams(); len(streams) == 1 {
			if streams[0].ProxyUser != "alice" || streams[0].Backend != "provider-a" {
				t.Errorf("active stream = %+v, want ProxyUser=alice Backend=provider-a", streams[0])
			}
			close(release)

			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("request did not complete after releasing upstream")
			}

			if got := len(c.ActiveStreams()); got != 0 {
				t.Errorf("ActiveStreams() after request finished = %d, want 0", got)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	close(release)
	t.Fatal("stream was never observed as active")
}

func TestLiveStream_StopForciblyEndsStream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		for {
			select {
			case <-r.Context().Done():
				return
			default:
				w.Write([]byte("x")) // nolint: errcheck
				flusher.Flush()
				time.Sleep(10 * time.Millisecond)
			}
		}
	}))
	defer upstream.Close()

	c := newTestServer(t)
	proxy := httptest.NewServer(c.Router)
	defer proxy.Close()

	xc, err := c.Store.CreateXtreamCode("provider-a", upstream.URL, "xuser", "xpass")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}
	if _, err := c.Store.CreateUser("alice", "hunter2", xc.ID, 0); err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}

	done := make(chan struct{})
	go func() {
		resp, err := http.Get(proxy.URL + "/live/alice/hunter2/42.ts")
		if err == nil {
			io.Copy(io.Discard, resp.Body) // nolint: errcheck
			resp.Body.Close()              // nolint: errcheck
		}
		close(done)
	}()

	var id int64
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if streams := c.ActiveStreams(); len(streams) == 1 {
			id = streams[0].ID
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if id == 0 {
		t.Fatal("stream never became active; nothing to stop")
	}

	// Without StopStream, this upstream handler never closes on its
	// own, simulating a player that never cleanly disconnects.
	if !c.StopStream(id) {
		t.Fatal("StopStream() returned false for an active stream")
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not complete after StopStream()")
	}

	if got := len(c.ActiveStreams()); got != 0 {
		t.Errorf("ActiveStreams() after StopStream() = %d, want 0", got)
	}

	if c.StopStream(id) {
		t.Error("StopStream() returned true for an already-ended stream")
	}
}

func TestXMLTV_NoLoginAndNoExtraActionParam(t *testing.T) {
	var gotURL *url.URL
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL
		w.Write([]byte("<tv></tv>")) // nolint: errcheck
	}))
	defer upstream.Close()

	c := newTestServer(t)

	xc, err := c.Store.CreateXtreamCode("provider-a", upstream.URL, "xuser", "xpass")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}
	if _, err := c.Store.CreateUser("alice", "hunter2", xc.ID, 0); err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}

	proxy := httptest.NewServer(c.Router)
	defer proxy.Close()

	resp, err := http.Get(proxy.URL + "/xmltv.php?username=alice&password=hunter2")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close() // nolint: errcheck
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", resp.StatusCode, body)
	}
	if string(body) != "<tv></tv>" {
		t.Errorf("body = %q, want %q", body, "<tv></tv>")
	}

	if gotURL == nil {
		t.Fatal("upstream was never contacted")
	}
	if gotURL.Path != "/xmltv.php" {
		t.Errorf("upstream path = %q, want %q (no separate login call expected)", gotURL.Path, "/xmltv.php")
	}
	if gotURL.Query().Get("action") != "" {
		t.Errorf("upstream query has action=%q, want no action param at all", gotURL.Query().Get("action"))
	}
	if gotURL.Query().Get("username") != "xuser" || gotURL.Query().Get("password") != "xpass" {
		t.Errorf("upstream query = %q, want upstream xtream credentials", gotURL.RawQuery)
	}
}

func TestAuth_RateLimitedAfterRepeatedFailures(t *testing.T) {
	c := newTestServer(t)
	proxy := httptest.NewServer(c.Router)
	defer proxy.Close()

	xc, err := c.Store.CreateXtreamCode("provider-a", "http://unused.example.com", "xuser", "xpass")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}
	if _, err := c.Store.CreateUser("alice", "hunter2", xc.ID, 0); err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}

	var lastStatus int
	for i := 0; i < authAttemptLimit+1; i++ {
		resp, err := http.Get(proxy.URL + "/live/alice/wrong-password/1.ts")
		if err != nil {
			t.Fatalf("GET error: %v", err)
		}
		resp.Body.Close() // nolint: errcheck
		lastStatus = resp.StatusCode
	}

	if lastStatus != http.StatusTooManyRequests {
		t.Errorf("status after %d failed attempts = %d, want %d", authAttemptLimit+1, lastStatus, http.StatusTooManyRequests)
	}

	// A correct login from the same IP is also blocked while the
	// window is still active — this is intentional: rate limiting
	// happens before credentials are even checked.
	resp, err := http.Get(proxy.URL + "/live/alice/hunter2/1.ts")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close() // nolint: errcheck
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status for correct login while blocked = %d, want %d", resp.StatusCode, http.StatusTooManyRequests)
	}
}

func TestPerUserStreamLimit_EvictsOldestOnNewStream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		for {
			select {
			case <-r.Context().Done():
				return
			default:
				w.Write([]byte("x")) // nolint: errcheck
				flusher.Flush()
				time.Sleep(10 * time.Millisecond)
			}
		}
	}))
	defer upstream.Close()

	c := newTestServer(t)
	proxy := httptest.NewServer(c.Router)
	defer proxy.Close()

	xc, err := c.Store.CreateXtreamCode("provider-a", upstream.URL, "xuser", "xpass")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}
	// Limit of 1: starting a second stream must evict the first.
	if _, err := c.Store.CreateUser("alice", "hunter2", xc.ID, 1); err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}

	firstDone := make(chan struct{})
	go func() {
		resp, err := http.Get(proxy.URL + "/live/alice/hunter2/1.ts")
		if err == nil {
			io.Copy(io.Discard, resp.Body) // nolint: errcheck
			resp.Body.Close()              // nolint: errcheck
		}
		close(firstDone)
	}()

	waitForStreamCount(t, c, 1)

	secondDone := make(chan struct{})
	go func() {
		resp, err := http.Get(proxy.URL + "/live/alice/hunter2/2.ts")
		if err == nil {
			io.Copy(io.Discard, resp.Body) // nolint: errcheck
			resp.Body.Close()              // nolint: errcheck
		}
		close(secondDone)
	}()

	select {
	case <-firstDone:
	case <-time.After(2 * time.Second):
		t.Fatal("first stream was not evicted after the second started")
	}

	streams := c.ActiveStreams()
	if len(streams) != 1 {
		t.Fatalf("ActiveStreams() len = %d, want 1 (limit is 1)", len(streams))
	}
	if streams[0].Path != "/live/alice/hunter2/2.ts" {
		t.Errorf("surviving stream path = %q, want the second request's path", streams[0].Path)
	}

	select {
	case <-secondDone:
		t.Fatal("second stream ended unexpectedly; it should still be running")
	default:
	}

	// Clean up the still-running second stream so the deferred
	// upstream.Close()/proxy.Close() (which block until all
	// outstanding connections finish) don't hang the test.
	c.StopStream(streams[0].ID)
	select {
	case <-secondDone:
	case <-time.After(2 * time.Second):
		t.Fatal("second stream did not stop after StopStream()")
	}
}

func waitForStreamCount(t *testing.T, c *Config, want int) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(c.ActiveStreams()) == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("ActiveStreams() never reached length %d", want)
}
