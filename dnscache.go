package main

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// dnsCacheEntry holds resolved IPs with expiration time
type dnsCacheEntry struct {
	ips      []net.IP
	expireAt time.Time
}

// dnsCache provides thread-safe caching of DNS lookups with TTL
type dnsCache struct {
	mu       sync.RWMutex
	entries  map[string]*dnsCacheEntry
	resolver *net.Resolver
	ttl      time.Duration
}

func newDNSCache(resolver *net.Resolver, ttl time.Duration) *dnsCache {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	cache := &dnsCache{
		entries:  make(map[string]*dnsCacheEntry),
		resolver: resolver,
		ttl:      ttl,
	}
	// Start background cleanup goroutine
	go cache.cleanup()
	return cache
}

// lookup resolves a host and caches the result
func (c *dnsCache) lookup(ctx context.Context, host string) ([]net.IP, error) {
	normalized := normalizeTargetHost(host)
	if normalized == "" {
		return nil, nil
	}

	// Fast path: check cache with read lock
	c.mu.RLock()
	entry, exists := c.entries[normalized]
	c.mu.RUnlock()

	if exists && time.Now().Before(entry.expireAt) {
		return entry.ips, nil
	}

	// Slow path: resolve and update cache
	ips, err := resolveTargetIPs(ctx, c.resolver, normalized)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.entries[normalized] = &dnsCacheEntry{
		ips:      ips,
		expireAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()

	return ips, nil
}

// cleanup removes expired entries every minute
func (c *dnsCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		c.mu.Lock()
		for host, entry := range c.entries {
			if now.After(entry.expireAt) {
				delete(c.entries, host)
			}
		}
		c.mu.Unlock()
	}
}

// resolveSafeTargetCached resolves target with caching
func (c *dnsCache) resolveSafeTargetCached(ctx context.Context, t *target) (*resolvedTarget, error) {
	normalized := normalizeTargetHost(t.Domain)

	// Check blocked hosts before lookup
	if normalized == "localhost" || normalized == "host.docker.internal" {
		return nil, fmt.Errorf("blocked target host: %s", t.Domain)
	}

	ips, err := c.lookup(ctx, t.Domain)
	if err != nil {
		return nil, err
	}

	// Filter unsafe IPs
	unsafeIPs := false
	safeIPs := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		if isDangerousIP(ip) {
			unsafeIPs = true
			continue
		}
		safeIPs = append(safeIPs, ip)
	}

	if unsafeIPs || len(safeIPs) == 0 {
		return nil, fmt.Errorf("blocked target host: %s", t.Domain)
	}

	return &resolvedTarget{dialAddrs: buildDialAddresses(safeIPs, t.Port)}, nil
}
