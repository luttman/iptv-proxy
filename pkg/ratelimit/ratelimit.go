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

// Package ratelimit provides a small per-key failure limiter, used to
// slow down credential brute-forcing without penalizing legitimate
// repeated successful requests (e.g. a real user's player hitting an
// auth-protected stream endpoint many times a day).
package ratelimit

import (
	"sync"
	"time"
)

type bucket struct {
	failures    int
	windowStart time.Time
}

// Limiter tracks failed-attempt counts per key (typically a client
// IP) within a rolling window. It only counts what callers explicitly
// report as failures via RecordFailure — successful requests should
// never be passed to it, so a busy legitimate caller is never
// throttled.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	limit   int
	window  time.Duration
}

// New returns a Limiter allowing up to limit failures per key within
// window before Blocked starts returning true for that key. It starts
// a background janitor goroutine that periodically evicts expired
// buckets, so it's meant to be created once per process (e.g. as a
// Config field), not per request.
func New(limit int, window time.Duration) *Limiter {
	l := &Limiter{
		buckets: map[string]*bucket{},
		limit:   limit,
		window:  window,
	}

	go l.janitor()

	return l
}

func (l *Limiter) janitor() {
	ticker := time.NewTicker(l.window)
	defer ticker.Stop()

	for range ticker.C {
		cutoff := time.Now().Add(-l.window)

		l.mu.Lock()
		for key, b := range l.buckets {
			if b.windowStart.Before(cutoff) {
				delete(l.buckets, key)
			}
		}
		l.mu.Unlock()
	}
}

// Blocked reports whether key has exceeded its failure quota within
// the current window.
func (l *Limiter) Blocked(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		return false
	}
	if time.Since(b.windowStart) > l.window {
		delete(l.buckets, key)
		return false
	}

	return b.failures >= l.limit
}

// RecordFailure increments key's failure count, starting a new window
// if the previous one has expired.
func (l *Limiter) RecordFailure(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok || time.Since(b.windowStart) > l.window {
		b = &bucket{windowStart: time.Now()}
		l.buckets[key] = b
	}

	b.failures++
}

// RecordSuccess clears key's failure count, so legitimate use right
// after a typo (or an attacker who happens to guess correctly) isn't
// penalized for prior failures.
func (l *Limiter) RecordSuccess(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.buckets, key)
}
