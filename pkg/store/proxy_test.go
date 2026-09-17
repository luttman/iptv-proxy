package store

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProxySettingsEncryptedAndRestored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "proxy.db")
	key := []byte(strings.Repeat("k", 32))
	st, err := Open(path, key)
	if err != nil {
		t.Fatal(err)
	}
	want := ProxySettings{Enabled: false, URL: "http://user:secret@proxy.example.com:3128"}
	if err := st.SaveProxySettings(want); err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := st.db.QueryRow(`SELECT value FROM settings WHERE name = 'outbound_proxy'`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(raw, encPrefix) || strings.Contains(raw, "secret") {
		t.Fatal("settings not encrypted")
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = Open(path, key)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close() // nolint: errcheck
	got, err := st.ProxySettings()
	if err != nil || got != want {
		t.Fatalf("restored settings = %+v, err = %v", got, err)
	}
}
