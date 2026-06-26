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
	"strings"
	"testing"

	"github.com/jamesnetherton/m3u"
	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/config"
)

func newTestConfig() *Config {
	return &Config{
		ProxyConfig: &config.ProxyConfig{
			HostConfig: &config.HostConfiguration{
				Hostname: "proxy.example.com",
				Port:     8080,
			},
			XtreamUser:     "xuser",
			XtreamPassword: "xpass",
			User:           "puser",
			Password:       "ppass",
			AdvertisedPort: 8080,
		},
		playlist:             &m3u.Playlist{},
		endpointAntiColision: "abc123",
	}
}

func TestReplaceURL_NonXtream(t *testing.T) {
	c := newTestConfig()

	got, err := c.replaceURL("http://origin.example.com/stream/channel.ts", 3, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "http://proxy.example.com:8080/abc123/puser/ppass/3/channel.ts"
	if got != want {
		t.Errorf("replaceURL() = %q, want %q", got, want)
	}
}

func TestReplaceURL_Xtream(t *testing.T) {
	c := newTestConfig()

	got, err := c.replaceURL("http://origin.example.com/xuser/xpass/123.ts", 0, true)
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
	c := newTestConfig()
	c.HTTPS = true

	got, err := c.replaceURL("http://origin.example.com/stream/channel.ts", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(got, "https://") {
		t.Errorf("replaceURL() = %q, want https scheme", got)
	}
}

func TestReplaceURL_CustomEndpoint(t *testing.T) {
	c := newTestConfig()
	c.CustomEndpoint = "/myendpoint/"

	got, err := c.replaceURL("http://origin.example.com/stream/channel.ts", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "http://proxy.example.com:8080/myendpoint/abc123/puser/ppass/0/channel.ts"
	if got != want {
		t.Errorf("replaceURL() = %q, want %q", got, want)
	}
}

func TestMarshallInto(t *testing.T) {
	c := newTestConfig()
	c.playlist = &m3u.Playlist{
		Tracks: []m3u.Track{
			{
				Name:   "Channel One",
				Length: -1,
				URI:    "http://origin.example.com/stream/one.ts",
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
	defer f.Close()

	if err := c.marshallInto(f, false); err != nil {
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
	if len(c.playlist.Tracks) != 1 {
		t.Errorf("marshallInto() filtered out a valid track, len=%d", len(c.playlist.Tracks))
	}
}

func TestMarshallInto_DropsTracksWithInvalidURI(t *testing.T) {
	c := newTestConfig()
	c.playlist = &m3u.Playlist{
		Tracks: []m3u.Track{
			{Name: "Bad", Length: -1, URI: "http://[::1]:namedport/bad"},
			{Name: "Good", Length: -1, URI: "http://origin.example.com/stream/good.ts"},
		},
	}

	f, err := os.CreateTemp(t.TempDir(), "playlist-*.m3u")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer f.Close()

	if err := c.marshallInto(f, false); err != nil {
		t.Fatalf("marshallInto() error: %v", err)
	}

	if len(c.playlist.Tracks) != 1 {
		t.Fatalf("expected 1 surviving track, got %d", len(c.playlist.Tracks))
	}
	if c.playlist.Tracks[0].Name != "Good" {
		t.Errorf("expected surviving track to be %q, got %q", "Good", c.playlist.Tracks[0].Name)
	}
}
