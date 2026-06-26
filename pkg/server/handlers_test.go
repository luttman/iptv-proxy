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
	"net/http"
	"testing"
)

func TestValuesContains(t *testing.T) {
	vs := values{"a", "b", "c"}

	if !vs.contains("b") {
		t.Error("expected values to contain \"b\"")
	}
	if vs.contains("z") {
		t.Error("expected values to not contain \"z\"")
	}
}

func TestMergeHttpHeader(t *testing.T) {
	dst := http.Header{}
	dst.Add("X-Shared", "dst-value")

	src := http.Header{}
	src.Add("X-Shared", "dst-value") // duplicate, should not be added twice
	src.Add("X-Shared", "src-value") // new value, should be appended
	src.Add("X-New", "new-value")

	mergeHttpHeader(dst, src)

	got := dst.Values("X-Shared")
	want := []string{"dst-value", "src-value"}
	if len(got) != len(want) {
		t.Fatalf("X-Shared = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("X-Shared[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	if dst.Get("X-New") != "new-value" {
		t.Errorf("X-New = %q, want %q", dst.Get("X-New"), "new-value")
	}
}
