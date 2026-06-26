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

import "testing"

func TestStreamTracker(t *testing.T) {
	tr := newStreamTracker()

	ru := testResolvedUser()
	id1 := tr.start(ru, "/live/alice/hunter2/1.ts", func() {})
	id2 := tr.start(ru, "/live/alice/hunter2/2.ts", func() {})

	if got := tr.list(); len(got) != 2 {
		t.Fatalf("list() len = %d, want 2", len(got))
	}

	tr.end(id1)

	got := tr.list()
	if len(got) != 1 {
		t.Fatalf("list() after end() len = %d, want 1", len(got))
	}
	if got[0].ID != id2 {
		t.Errorf("remaining stream ID = %d, want %d", got[0].ID, id2)
	}
	if got[0].ProxyUser != ru.ProxyUser || got[0].Backend != ru.Backend.Name {
		t.Errorf("remaining stream = %+v, want ProxyUser/Backend to match %+v", got[0], ru)
	}

	tr.end(id2)
	if got := tr.list(); len(got) != 0 {
		t.Errorf("list() after ending all streams len = %d, want 0", len(got))
	}
}

func TestStreamTracker_Stop(t *testing.T) {
	tr := newStreamTracker()

	var canceled bool
	id := tr.start(testResolvedUser(), "/live/alice/hunter2/1.ts", func() { canceled = true })

	if !tr.stop(id) {
		t.Fatal("stop() returned false for an active stream")
	}
	if !canceled {
		t.Error("stop() did not invoke the stream's cancel func")
	}

	// stop() only invokes cancel; it's end() (called by the stream's
	// own deferred cleanup once cancellation unblocks it) that removes
	// the entry. So the entry is still present, and stop() again is
	// a harmless no-op (context.CancelFunc is safe to call repeatedly).
	if !tr.stop(id) {
		t.Error("stop() returned false on a still-registered stream")
	}

	tr.end(id)
	if tr.stop(999) {
		t.Error("stop() returned true for a nonexistent stream id")
	}
}
