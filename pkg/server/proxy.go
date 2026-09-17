package server

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/pierre-emmanuelJ/iptv-proxy/pkg/store"
)

type outboundProxy struct {
	sync.RWMutex
	settings store.ProxySettings
}

func validateProxyURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, errors.New("invalid proxy URL")
	}
	switch u.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return nil, errors.New("proxy URL must start with http://, https://, or socks5://")
	}
	if u.Hostname() == "" || u.Opaque != "" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("proxy URL must contain a host and optional port, without a path, query, or fragment")
	}
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 {
			return nil, errors.New("proxy port must be between 1 and 65535")
		}
	}
	return u, nil
}

func (p *outboundProxy) proxy(req *http.Request) (*url.URL, error) {
	p.RLock()
	s := p.settings
	p.RUnlock()
	if !s.Enabled {
		return nil, nil
	}
	if s.UseEnvironment {
		return http.ProxyFromEnvironment(req)
	}
	return validateProxyURL(s.URL)
}

func (c *Config) ProxySettings() store.ProxySettings {
	c.outboundProxy.RLock()
	defer c.outboundProxy.RUnlock()
	return c.outboundProxy.settings
}

// SetProxySettings saves first, then applies to new requests. Existing streams
// keep their connection; idle connections are discarded.
func (c *Config) SetProxySettings(s store.ProxySettings) error {
	s.URL = strings.TrimSpace(s.URL)
	if s.URL != "" {
		if _, err := validateProxyURL(s.URL); err != nil {
			return err
		}
	} else if !s.UseEnvironment && s.Enabled {
		return errors.New("enter a proxy URL before enabling a custom proxy")
	}
	c.outboundProxy.Lock()
	defer c.outboundProxy.Unlock()
	if err := c.Store.SaveProxySettings(s); err != nil {
		return err
	}
	c.outboundProxy.settings = s
	c.httpClient.CloseIdleConnections()
	return nil
}
