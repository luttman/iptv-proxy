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

package ratelimit

import (
	"testing"
	"time"
)

func TestLimiter_BlocksAfterLimit(t *testing.T) {
	l := New(3, time.Minute)

	for i := 0; i < 3; i++ {
		if l.Blocked("1.2.3.4") {
			t.Fatalf("Blocked() = true after %d failures, want false (limit is 3)", i)
		}
		l.RecordFailure("1.2.3.4")
	}

	if !l.Blocked("1.2.3.4") {
		t.Error("Blocked() = false after hitting the limit, want true")
	}
}

func TestLimiter_DistinctKeysIndependent(t *testing.T) {
	l := New(1, time.Minute)

	l.RecordFailure("1.2.3.4")
	if !l.Blocked("1.2.3.4") {
		t.Error("Blocked(1.2.3.4) = false, want true")
	}
	if l.Blocked("5.6.7.8") {
		t.Error("Blocked(5.6.7.8) = true, want false (different key)")
	}
}

func TestLimiter_SuccessClearsFailures(t *testing.T) {
	l := New(1, time.Minute)

	l.RecordFailure("1.2.3.4")
	if !l.Blocked("1.2.3.4") {
		t.Fatal("Blocked() = false, want true after one failure (limit is 1)")
	}

	l.RecordSuccess("1.2.3.4")
	if l.Blocked("1.2.3.4") {
		t.Error("Blocked() = true after RecordSuccess(), want false")
	}
}

func TestLimiter_WindowExpiry(t *testing.T) {
	l := New(1, 20*time.Millisecond)

	l.RecordFailure("1.2.3.4")
	if !l.Blocked("1.2.3.4") {
		t.Fatal("Blocked() = false, want true immediately after a failure")
	}

	time.Sleep(40 * time.Millisecond)

	if l.Blocked("1.2.3.4") {
		t.Error("Blocked() = true after the window expired, want false")
	}
}
