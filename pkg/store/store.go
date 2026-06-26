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

// Package store provides SQLite-backed persistence for proxy users
// and the xtream-code backends they're assigned to.
package store

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// Store wraps a SQLite database holding users and xtream codes.
type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the SQLite database at path and
// ensures the schema exists.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	// SQLite does not support concurrent writers; a single connection
	// avoids "database is locked" errors under concurrent requests.
	db.SetMaxOpenConns(1)

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS xtream_codes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	base_url TEXT NOT NULL,
	xtream_user TEXT NOT NULL,
	xtream_password TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	xtream_code_id INTEGER NOT NULL REFERENCES xtream_codes(id),
	max_concurrent_streams INTEGER NOT NULL DEFAULT 1,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("migrate schema: %w", err)
	}

	// Defensive migration for databases created before
	// max_concurrent_streams existed; CREATE TABLE IF NOT EXISTS above
	// doesn't add columns to an already-existing table.
	if _, err := s.db.Exec(`ALTER TABLE users ADD COLUMN max_concurrent_streams INTEGER NOT NULL DEFAULT 1`); err != nil {
		if !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("migrate schema (add max_concurrent_streams): %w", err)
		}
	}

	return nil
}
