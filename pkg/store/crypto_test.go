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

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func testKey(b byte) []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = b
	}
	return key
}

func TestCredentialsEncryptedAtRestAndDecryptedOnRead(t *testing.T) {
	key := testKey(1)
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path, key)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	xc, err := s.CreateXtreamCode("provider-a", "http://a.example.com", "secretuser", "secretpass")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}
	s.Close() // nolint: errcheck

	// The raw stored value must not contain the plaintext credentials.
	rawDB, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error: %v", err)
	}
	var rawUser, rawPass string
	row := rawDB.QueryRow(`SELECT xtream_user, xtream_password FROM xtream_codes WHERE id = ?`, xc.ID)
	if err := row.Scan(&rawUser, &rawPass); err != nil {
		t.Fatalf("scan raw row: %v", err)
	}
	rawDB.Close() // nolint: errcheck
	if rawUser == "secretuser" || rawPass == "secretpass" {
		t.Fatal("credentials were stored in plaintext")
	}

	s2, err := Open(path, key)
	if err != nil {
		t.Fatalf("re-Open() with key error: %v", err)
	}
	defer s2.Close() // nolint: errcheck

	got, err := s2.GetXtreamCode(xc.ID)
	if err != nil {
		t.Fatalf("GetXtreamCode() error: %v", err)
	}
	if got.XtreamUser != "secretuser" || got.XtreamPassword != "secretpass" {
		t.Errorf("GetXtreamCode() = %+v, want decrypted credentials", got)
	}
}

func TestOpenRefusesMissingOrWrongKey(t *testing.T) {
	key := testKey(1)
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path, key)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	if _, err := s.CreateXtreamCode("provider-a", "http://a.example.com", "u", "p"); err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}
	s.Close() // nolint: errcheck

	if _, err := Open(path, nil); err == nil {
		t.Error("Open() with no key on an encrypted database: want error, got nil")
	}

	if _, err := Open(path, testKey(2)); !errors.Is(err, ErrEncryptionKeyRequired) {
		t.Errorf("Open() with wrong key: err = %v, want ErrEncryptionKeyRequired", err)
	}
}

func TestEncryptExistingCredentialsMigratesInTransaction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path, nil)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	xcA, err := s.CreateXtreamCode("provider-a", "http://a.example.com", "userA", "passA")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}
	xcB, err := s.CreateXtreamCode("provider-b", "http://b.example.com", "userB", "passB")
	if err != nil {
		t.Fatalf("CreateXtreamCode() error: %v", err)
	}

	key := testKey(9)
	if err := s.EncryptExistingCredentials(key); err != nil {
		t.Fatalf("EncryptExistingCredentials() error: %v", err)
	}
	s.Close() // nolint: errcheck

	s2, err := Open(path, key)
	if err != nil {
		t.Fatalf("re-Open() with key error: %v", err)
	}
	defer s2.Close() // nolint: errcheck

	gotA, err := s2.GetXtreamCode(xcA.ID)
	if err != nil || gotA.XtreamUser != "userA" || gotA.XtreamPassword != "passA" {
		t.Errorf("GetXtreamCode(A) = %+v, err %v, want decrypted userA/passA", gotA, err)
	}
	gotB, err := s2.GetXtreamCode(xcB.ID)
	if err != nil || gotB.XtreamUser != "userB" || gotB.XtreamPassword != "passB" {
		t.Errorf("GetXtreamCode(B) = %+v, err %v, want decrypted userB/passB", gotB, err)
	}

	// Running the migration again should be a harmless no-op.
	if err := s2.EncryptExistingCredentials(key); err != nil {
		t.Fatalf("EncryptExistingCredentials() second run error: %v", err)
	}
}
