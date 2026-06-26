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

// XtreamCode is an upstream xtream-codes backend that one or more
// users can be assigned to.
type XtreamCode struct {
	ID             int64
	Name           string
	BaseURL        string
	XtreamUser     string
	XtreamPassword string
	CreatedAt      time.Time
}

// User is a proxy-facing login, assigned to exactly one XtreamCode.
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	XtreamCodeID int64
	CreatedAt    time.Time
}
