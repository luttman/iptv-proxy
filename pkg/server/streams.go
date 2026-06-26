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
	"sort"
	"sync"
	"time"
)

// ActiveStream describes a single in-flight proxied stream, for
// display in the admin UI.
type ActiveStream struct {
	ID        int64
	ProxyUser string
	Backend   string
	Path      string
	StartedAt time.Time
}

type streamEntry struct {
	ActiveStream
	cancel func()
}

// streamTracker counts and describes currently in-flight proxied
// streams (live channels, VOD, series, timeshift). A stream is
// tracked for exactly the duration the proxy is actively copying
// bytes from the upstream backend to the client.
//
// Some IPTV players don't cleanly close the old connection when
// switching channels, leaving a stream that looks "active" forever
// even though nothing is watching it. Each tracked stream carries a
// cancel func so it can be forcibly torn down from the admin UI,
// which both clears the stale entry and frees the connection slot on
// providers with a concurrent-connection cap.
type streamTracker struct {
	mu      sync.Mutex
	nextID  int64
	streams map[int64]streamEntry
}

func newStreamTracker() *streamTracker {
	return &streamTracker{streams: map[int64]streamEntry{}}
}

func (t *streamTracker) start(ru resolvedUser, path string, cancel func()) int64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.nextID++
	id := t.nextID
	t.streams[id] = streamEntry{
		ActiveStream: ActiveStream{
			ID:        id,
			ProxyUser: ru.ProxyUser,
			Backend:   ru.Backend.Name,
			Path:      path,
			StartedAt: time.Now(),
		},
		cancel: cancel,
	}

	return id
}

func (t *streamTracker) end(id int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.streams, id)
}

// stop cancels the in-flight stream identified by id, if any. The
// stream's own deferred cleanup (in stream()) removes it from the
// tracker once the cancellation unblocks the copy loop.
func (t *streamTracker) stop(id int64) bool {
	t.mu.Lock()
	entry, ok := t.streams[id]
	t.mu.Unlock()

	if !ok {
		return false
	}

	entry.cancel()

	return true
}

func (t *streamTracker) list() []ActiveStream {
	t.mu.Lock()
	defer t.mu.Unlock()

	out := make([]ActiveStream, 0, len(t.streams))
	for _, s := range t.streams {
		out = append(out, s.ActiveStream)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.Before(out[j].StartedAt) })

	return out
}

// ActiveStreams returns a snapshot of all currently in-flight
// proxied streams, oldest first.
func (c *Config) ActiveStreams() []ActiveStream {
	return c.streams.list()
}

// StopStream forcibly tears down the in-flight stream identified by
// id, if it's still active. Returns false if no such stream exists
// (e.g. it already ended naturally).
func (c *Config) StopStream(id int64) bool {
	return c.streams.stop(id)
}
