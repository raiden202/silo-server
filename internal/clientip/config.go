package clientip

import (
	"context"
	"fmt"
	"net"
	"strings"
)

// SettingsStore is satisfied by *catalog.ServerSettingsRepo.
type SettingsStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	GetAll(ctx context.Context) (map[string]string, error)
}

const (
	keyTrustedProxies = "clientip.trusted_proxies"
	// Default: RFC 1918 private ranges + loopback + link-local
	defaultTrustedProxies = "10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,127.0.0.0/8,::1/128"
)

// SeedDefaults writes the default trusted proxy config if not already set.
func SeedDefaults(ctx context.Context, store SettingsStore) error {
	existing, err := store.Get(ctx, keyTrustedProxies)
	if err != nil {
		return fmt.Errorf("seed clientip defaults: %w", err)
	}
	if existing != "" {
		return nil
	}
	return store.Set(ctx, keyTrustedProxies, defaultTrustedProxies)
}

// LoadTrustedCIDRs reads the trusted proxy CIDRs from settings.
func LoadTrustedCIDRs(ctx context.Context, store SettingsStore) ([]*net.IPNet, error) {
	raw, err := store.Get(ctx, keyTrustedProxies)
	if err != nil {
		return nil, fmt.Errorf("load trusted proxies: %w", err)
	}
	if raw == "" {
		raw = defaultTrustedProxies
	}
	return ParseCIDRs(raw)
}

// ParseCIDRs parses a comma-separated list of CIDR strings.
func ParseCIDRs(raw string) ([]*net.IPNet, error) {
	parts := strings.Split(raw, ",")
	var cidrs []*net.IPNet
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		_, cidr, err := net.ParseCIDR(p)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q: %w", p, err)
		}
		cidrs = append(cidrs, cidr)
	}
	return cidrs, nil
}
