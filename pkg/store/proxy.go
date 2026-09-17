package store

import (
	"database/sql"
	"encoding/json"
	"errors"
)

// ProxySettings controls outbound HTTP requests configured through the admin UI.
type ProxySettings struct {
	Enabled bool
	URL     string
}

func (s *Store) ProxySettings() (ProxySettings, error) {
	settings := ProxySettings{}
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE name = 'outbound_proxy'`).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return settings, nil
	}
	if err != nil {
		return settings, err
	}
	value, err = decryptValue(s.key, value)
	if err != nil {
		return settings, err
	}
	var saved struct {
		ProxySettings
		UseEnvironment bool // Read legacy settings without activating an unused custom URL.
	}
	err = json.Unmarshal([]byte(value), &saved)
	if saved.UseEnvironment {
		return settings, err
	}
	return saved.ProxySettings, err
}

func (s *Store) SaveProxySettings(settings ProxySettings) error {
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	value, err := encryptValue(s.key, string(data))
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO settings(name, value) VALUES('outbound_proxy', ?) ON CONFLICT(name) DO UPDATE SET value = excluded.value`, value)
	return err
}
