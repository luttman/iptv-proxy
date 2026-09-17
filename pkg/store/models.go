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

package store

import "time"

// XtreamCode is a named upstream provider: a group of one or more
// addresses and one or more credentials. A proxy user is assigned to
// one specific credential, not to the provider directly.
type XtreamCode struct {
	ID        int64
	Name      string
	CreatedAt time.Time
}

// XtreamAddress is one upstream URL under a provider. Enabled
// controls whether failover/health-checking considers it at all;
// disabling one doesn't delete its history.
type XtreamAddress struct {
	ID           int64
	XtreamCodeID int64
	Address      string
	Enabled      bool
}

// XtreamCredential is one xtream username/password pair under a
// provider. Several proxy users may share a credential (subject to
// the provider's own connection limit) or each get their own.
type XtreamCredential struct {
	ID             int64
	XtreamCodeID   int64
	XtreamUser     string
	XtreamPassword string
	CreatedAt      time.Time
}

// ResolvedBackend is the single, concrete backend a proxy user's
// credential resolves to for one request: a provider name, one
// selected (or full, space-separated) set of addresses, and the
// specific xtream username/password to authenticate with upstream.
type ResolvedBackend struct {
	ID             int64 // the XtreamCode (provider) ID
	Name           string
	BaseURL        string
	XtreamUser     string
	XtreamPassword string
}

// User is a proxy-facing login, assigned to exactly one
// XtreamCredential. MaxConcurrentStreams caps how many streams this
// user may have open at once; 0 means unlimited.
type User struct {
	ID                   int64
	Username             string
	PasswordHash         string
	CredentialID         int64
	MaxConcurrentStreams int
	CreatedAt            time.Time
}
