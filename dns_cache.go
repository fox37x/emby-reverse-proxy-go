package main

import (
	"context"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultDNSCacheTTL = 60 * time.Second

type targetCacheEntry struct {
	rt        *resolvedTarget
	expiresAt time.Time
}

type targetResolverCache struct {
	resolver *net.Resolver
	ttl      time.Duration
	entries  sync.Map
}

func newTargetResolverCache(resolver *net.Resolver, ttl time.Duration) *targetResolverCache {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	return &targetResolverCache{resolver: resolver, ttl: ttl}
}

func parseDNSCacheTTL(raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultDNSCacheTTL
	}
	if seconds, err := strconv.Atoi(raw); err == nil {
		if seconds <= 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	if d, err := time.ParseDuration(raw); err == nil && d > 0 {
		return d
	}
	return defaultDNSCacheTTL
}

func targetCacheKey(t *target) string {
	return t.Scheme + "://" + normalizeTargetHost(t.Domain) + ":" + strconv.Itoa(t.Port)
}

func cloneResolvedTarget(rt *resolvedTarget) *resolvedTarget {
	if rt == nil {
		return nil
	}
	return &resolvedTarget{dialAddrs: append([]string(nil), rt.dialAddrs...)}
}

func (c *targetResolverCache) resolve(ctx context.Context, t *target) (*resolvedTarget, error) {
	if c == nil || c.ttl <= 0 {
		return resolveSafeTarget(ctx, net.DefaultResolver, t)
	}

	key := targetCacheKey(t)
	now := time.Now()
	if value, ok := c.entries.Load(key); ok {
		entry := value.(targetCacheEntry)
		if now.Before(entry.expiresAt) {
			return cloneResolvedTarget(entry.rt), nil
		}
		c.entries.Delete(key)
	}

	rt, err := resolveSafeTarget(ctx, c.resolver, t)
	if err != nil {
		return nil, err
	}
	c.entries.Store(key, targetCacheEntry{
		rt:        cloneResolvedTarget(rt),
		expiresAt: now.Add(c.ttl),
	})
	return rt, nil
}
