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
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jamesnetherton/m3u"
	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/config"
	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/store"
)

func newTestConfig(t *testing.T) *Config {
	t.Helper()

	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"), nil)
	if err != nil {
		t.Fatalf("store.Open() error: %v", err)
	}
	t.Cleanup(func() { st.Close() }) // nolint: errcheck

	return &Config{
		ProxyConfig: &config.ProxyConfig{
			HostConfig: &config.HostConfiguration{
				Hostname: "proxy.example.com",
				Port:     8080,
			},
			AdvertisedPort: 8080,
		},
		Store:                  st,
		hlsChannelsRedirectURL: map[string]hlsRedirect{},
		xtreamM3uCache:         map[string]cacheMeta{},
		upstreamHealth:         map[string]UpstreamHealth{},
	}
}

func testResolvedUser() resolvedUser {
	return resolvedUser{
		ProxyUser:     "puser",
		ProxyPassword: "ppass",
		Backend: store.XtreamCode{
			ID:             1,
			Name:           "provider-a",
			BaseURL:        "http://origin.example.com",
			XtreamUser:     "xuser",
			XtreamPassword: "xpass",
		},
	}
}

func TestReplaceURL_Xtream(t *testing.T) {
	c := newTestConfig(t)
	ru := testResolvedUser()

	got, err := c.replaceURL("http://origin.example.com/xuser/xpass/123.ts", ru)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(got, "xuser") || strings.Contains(got, "xpass") {
		t.Errorf("replaceURL() = %q, expected xtream credentials to be replaced", got)
	}
	if !strings.Contains(got, "puser") || !strings.Contains(got, "ppass") {
		t.Errorf("replaceURL() = %q, expected proxy credentials to be present", got)
	}
}

func TestReplaceURL_HTTPS(t *testing.T) {
	c := newTestConfig(t)
	c.HTTPS = true
	ru := testResolvedUser()

	got, err := c.replaceURL("http://origin.example.com/xuser/xpass/123.ts", ru)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(got, "https://") {
		t.Errorf("replaceURL() = %q, want https scheme", got)
	}
}

func TestReplaceURL_CustomEndpoint(t *testing.T) {
	c := newTestConfig(t)
	c.CustomEndpoint = "/myendpoint/"
	ru := testResolvedUser()

	got, err := c.replaceURL("http://origin.example.com/xuser/xpass/123.ts", ru)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "http://proxy.example.com:8080/myendpoint/puser/ppass/123.ts"
	if got != want {
		t.Errorf("replaceURL() = %q, want %q", got, want)
	}
}

func TestMarshallInto(t *testing.T) {
	c := newTestConfig(t)
	ru := testResolvedUser()
	playlist := &m3u.Playlist{
		Tracks: []m3u.Track{
			{
				Name:   "Channel One",
				Length: -1,
				URI:    "http://origin.example.com/xuser/xpass/one.ts",
				Tags: []m3u.Tag{
					{Name: "tvg-id", Value: "one"},
				},
			},
		},
	}

	f, err := os.CreateTemp(t.TempDir(), "playlist-*.m3u")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer f.Close() // nolint: errcheck

	if err := c.marshallInto(f, playlist, ru); err != nil {
		t.Fatalf("marshallInto() error: %v", err)
	}

	contents, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}

	if !bytes.HasPrefix(contents, []byte("#EXTM3U\n")) {
		t.Errorf("marshallInto() output missing #EXTM3U header, got %q", contents)
	}
	if !strings.Contains(string(contents), "Channel One") {
		t.Errorf("marshallInto() output missing track name, got %q", contents)
	}
	if !strings.Contains(string(contents), "proxy.example.com:8080") {
		t.Errorf("marshallInto() output missing rewritten host, got %q", contents)
	}
	if len(playlist.Tracks) != 1 {
		t.Errorf("marshallInto() filtered out a valid track, len=%d", len(playlist.Tracks))
	}
}

func TestMarshallInto_DropsTracksWithInvalidURI(t *testing.T) {
	c := newTestConfig(t)
	ru := testResolvedUser()
	playlist := &m3u.Playlist{
		Tracks: []m3u.Track{
			{Name: "Bad", Length: -1, URI: "http://[::1]:namedport/bad"},
			{Name: "Good", Length: -1, URI: "http://origin.example.com/xuser/xpass/good.ts"},
		},
	}

	f, err := os.CreateTemp(t.TempDir(), "playlist-*.m3u")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer f.Close() // nolint: errcheck

	if err := c.marshallInto(f, playlist, ru); err != nil {
		t.Fatalf("marshallInto() error: %v", err)
	}

	if len(playlist.Tracks) != 1 {
		t.Fatalf("expected 1 surviving track, got %d", len(playlist.Tracks))
	}
	if playlist.Tracks[0].Name != "Good" {
		t.Errorf("expected surviving track to be %q, got %q", "Good", playlist.Tracks[0].Name)
	}
}
