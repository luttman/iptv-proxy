package store

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyProxySettings(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "legacy.db"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close() // nolint: errcheck
	for _, environment := range []bool{false, true} {
		raw := fmt.Sprintf(`{"Enabled":true,"UseEnvironment":%t,"URL":"http://proxy.example.com:3128"}`, environment)
		if _, err := st.db.Exec(`INSERT INTO settings(name,value) VALUES('outbound_proxy',?) ON CONFLICT(name) DO UPDATE SET value=excluded.value`, raw); err != nil {
			t.Fatal(err)
		}
		got, err := st.ProxySettings()
		if err != nil {
			t.Fatal(err)
		}
		if environment && (got.Enabled || got.URL != "") {
			t.Fatal("legacy environment mode activated a custom proxy")
		}
		if !environment && (!got.Enabled || got.URL != "http://proxy.example.com:3128") {
			t.Fatal("legacy admin proxy settings lost")
		}
	}
}

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
