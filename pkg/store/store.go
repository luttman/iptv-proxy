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
	"errors"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// Store wraps a SQLite database holding users and xtream codes.
type Store struct {
	db  *sql.DB
	key []byte
}

// Open opens (creating if needed) the SQLite database at path and
// ensures the schema exists. key, if non-nil, is a 32-byte AES-256
// key used to encrypt/decrypt xtream-code credentials at rest; pass
// nil to store credentials in plaintext (or to read a database that
// has none encrypted yet). Open refuses to return a Store if the
// database holds encrypted credentials but key is nil or incorrect.
func Open(path string, key []byte) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	// SQLite does not support concurrent writers; a single connection
	// avoids "database is locked" errors under concurrent requests.
	db.SetMaxOpenConns(1)

	s := &Store{db: db, key: key}
	if err := s.migrate(); err != nil {
		db.Close() // nolint: errcheck
		return nil, err
	}

	if err := s.checkEncryptionKey(); err != nil {
		db.Close() // nolint: errcheck
		return nil, err
	}

	if err := s.backfillAddressesAndCredentials(); err != nil {
		db.Close() // nolint: errcheck
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
CREATE TABLE IF NOT EXISTS settings (
	name TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

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

CREATE TABLE IF NOT EXISTS xtream_addresses (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	xtream_code_id INTEGER NOT NULL REFERENCES xtream_codes(id),
	address TEXT NOT NULL,
	enabled INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_xtream_addresses_code ON xtream_addresses(xtream_code_id);

CREATE TABLE IF NOT EXISTS xtream_credentials (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	xtream_code_id INTEGER NOT NULL REFERENCES xtream_codes(id),
	xtream_user TEXT NOT NULL,
	xtream_password TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_xtream_credentials_code ON xtream_credentials(xtream_code_id);

CREATE TABLE IF NOT EXISTS upstream_checks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	xtream_code_id INTEGER NOT NULL,
	address TEXT NOT NULL,
	checked_at TIMESTAMP NOT NULL,
	up INTEGER NOT NULL,
	latency_ms INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_upstream_checks_lookup ON upstream_checks(xtream_code_id, address, checked_at);

CREATE TABLE IF NOT EXISTS bandwidth_samples (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	sampled_at TIMESTAMP NOT NULL,
	bytes INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_bandwidth_samples_time ON bandwidth_samples(sampled_at);
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

	// A user used to be assigned directly to an xtream_code (one
	// address list, one credential). Now a provider can hold several
	// credentials, so a user is assigned to one specific credential
	// instead; xtream_code_id is kept (and still populated) purely so
	// existing rows/constraints stay intact.
	if _, err := s.db.Exec(`ALTER TABLE users ADD COLUMN credential_id INTEGER NOT NULL DEFAULT 0`); err != nil {
		if !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("migrate schema (add credential_id): %w", err)
		}
	}

	// Optional human-friendly label for a credential, shown instead of
	// the raw xtream username where there's room to.
	if _, err := s.db.Exec(`ALTER TABLE xtream_credentials ADD COLUMN name TEXT NOT NULL DEFAULT ''`); err != nil {
		if !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("migrate schema (add credential name): %w", err)
		}
	}

	return nil
}

// backfillAddressesAndCredentials copies each pre-migration
// xtream_code's single base_url/xtream_user/xtream_password into the
// new xtream_addresses/xtream_credentials tables, and points any user
// still on the old xtream_code_id-only assignment at the migrated
// credential. It's idempotent: a provider or user already migrated
// (has address/credential rows, or a non-zero credential_id) is left
// alone, so this is safe to run on every startup.
func (s *Store) backfillAddressesAndCredentials() error {
	rows, err := s.db.Query(`SELECT id, base_url, xtream_user, xtream_password FROM xtream_codes`)
	if err != nil {
		return fmt.Errorf("backfill: %w", err)
	}

	type legacyCode struct {
		id                  int64
		baseURL, user, pass string
	}
	var codes []legacyCode
	for rows.Next() {
		var c legacyCode
		if err := rows.Scan(&c.id, &c.baseURL, &c.user, &c.pass); err != nil {
			rows.Close() // nolint: errcheck
			return fmt.Errorf("backfill: %w", err)
		}
		codes = append(codes, c)
	}
	if err := rows.Err(); err != nil {
		rows.Close() // nolint: errcheck
		return fmt.Errorf("backfill: %w", err)
	}
	rows.Close() // nolint: errcheck

	for _, c := range codes {
		var addrCount int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM xtream_addresses WHERE xtream_code_id = ?`, c.id).Scan(&addrCount); err != nil {
			return fmt.Errorf("backfill: %w", err)
		}
		if addrCount == 0 {
			for _, addr := range strings.Fields(c.baseURL) {
				addr = strings.TrimRight(addr, "/")
				if addr == "" {
					continue
				}
				if _, err := s.db.Exec(`INSERT INTO xtream_addresses (xtream_code_id, address, enabled) VALUES (?, ?, 1)`, c.id, addr); err != nil {
					return fmt.Errorf("backfill: %w", err)
				}
			}
		}

		var credentialID int64
		if err := s.db.QueryRow(`SELECT id FROM xtream_credentials WHERE xtream_code_id = ? ORDER BY id LIMIT 1`, c.id).Scan(&credentialID); err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("backfill: %w", err)
			}
			if c.user == "" {
				continue
			}
			// c.user/c.pass are copied verbatim (already encrypted, if
			// encryption is configured) rather than decrypted and
			// re-encrypted: their on-disk form is exactly what a
			// credential row should hold too.
			res, err := s.db.Exec(`INSERT INTO xtream_credentials (xtream_code_id, xtream_user, xtream_password) VALUES (?, ?, ?)`, c.id, c.user, c.pass)
			if err != nil {
				return fmt.Errorf("backfill: %w", err)
			}
			credentialID, err = res.LastInsertId()
			if err != nil {
				return fmt.Errorf("backfill: %w", err)
			}
		}

		if credentialID != 0 {
			if _, err := s.db.Exec(`UPDATE users SET credential_id = ? WHERE xtream_code_id = ? AND credential_id = 0`, credentialID, c.id); err != nil {
				return fmt.Errorf("backfill: %w", err)
			}
		}
	}

	return nil
}

// checkEncryptionKey verifies that every stored credential can be
// decrypted with s.key, refusing to proceed otherwise. This turns a
// missing/wrong key into a startup failure instead of silent garbage
// credentials being used against upstream backends. It checks both
// the legacy xtream_codes columns (the source backfill copies from,
// on databases not yet migrated) and xtream_credentials (where
// credentials live from here on).
func (s *Store) checkEncryptionKey() error {
	for _, table := range []string{"xtream_codes", "xtream_credentials"} {
		rows, err := s.db.Query(`SELECT xtream_user, xtream_password FROM ` + table)
		if err != nil {
			return fmt.Errorf("check encryption key: %w", err)
		}

		for rows.Next() {
			var user, pass string
			if err := rows.Scan(&user, &pass); err != nil {
				rows.Close() // nolint: errcheck
				return fmt.Errorf("check encryption key: %w", err)
			}
			if _, err := decryptValue(s.key, user); err != nil {
				rows.Close() // nolint: errcheck
				return err
			}
			if _, err := decryptValue(s.key, pass); err != nil {
				rows.Close() // nolint: errcheck
				return err
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close() // nolint: errcheck
			return fmt.Errorf("check encryption key: %w", err)
		}
		rows.Close() // nolint: errcheck
	}

	return nil
}
