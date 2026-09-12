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
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// encPrefix marks a stored value as AES-GCM encrypted (base64 after
// the prefix); anything without it is treated as legacy plaintext.
const encPrefix = "enc:v1:"

// ErrEncryptionKeyRequired is returned when a value carries the
// encrypted-value marker but no key, or the wrong key, is available
// to decrypt it.
var ErrEncryptionKeyRequired = errors.New("credential is encrypted but the encryption key is missing or incorrect")

// LoadEncryptionKey reads a 32-byte AES-256 key from keyFile if set,
// else from keyEnv (raw env var value) if non-empty. The key may be
// base64-encoded or exactly 32 raw bytes. Returns a nil key (not an
// error) when neither source is set, meaning credentials are stored
// in plaintext.
func LoadEncryptionKey(keyFile, keyEnv string) ([]byte, error) {
	var raw string
	switch {
	case keyFile != "":
		b, err := os.ReadFile(keyFile)
		if err != nil {
			return nil, fmt.Errorf("read encryption key file: %w", err)
		}
		raw = strings.TrimSpace(string(b))
	case keyEnv != "":
		raw = strings.TrimSpace(keyEnv)
	default:
		return nil, nil
	}

	if len(raw) == 32 {
		return []byte(raw), nil
	}

	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, errors.New("encryption key must be 32 bytes, either raw or base64-encoded")
	}

	return key, nil
}

func encryptValue(key []byte, plaintext string) (string, error) {
	if key == nil {
		return plaintext, nil
	}

	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return encPrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decryptValue(key []byte, stored string) (string, error) {
	if !strings.HasPrefix(stored, encPrefix) {
		return stored, nil
	}
	if key == nil {
		return "", ErrEncryptionKeyRequired
	}

	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, encPrefix))
	if err != nil {
		return "", fmt.Errorf("decode encrypted value: %w", err)
	}

	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", ErrEncryptionKeyRequired
	}

	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrEncryptionKeyRequired
	}

	return string(plaintext), nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("init cipher: %w", err)
	}
	return cipher.NewGCM(block)
}
