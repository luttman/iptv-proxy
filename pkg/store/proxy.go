package store

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/config"
)

// ProxySettings controls outbound HTTP requests. Environment settings are the
// default until an administrator saves an override.
type ProxySettings struct {
	Enabled        bool
	UseEnvironment bool
	URL            string
}

func (s *Store) ProxySettings() (ProxySettings, error) {
	settings := ProxySettings{Enabled: config.HasEnvironmentProxy(), UseEnvironment: true}
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
	err = json.Unmarshal([]byte(value), &settings)
	return settings, err
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
