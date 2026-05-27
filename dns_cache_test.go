package main

import (
	"net"
	"testing"
	"time"
)

func TestParseDNSCacheTTL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want time.Duration
	}{
		{name: "default", raw: "", want: defaultDNSCacheTTL},
		{name: "seconds", raw: "5", want: 5 * time.Second},
		{name: "duration", raw: "2m", want: 2 * time.Minute},
		{name: "disabled", raw: "0", want: 0},
		{name: "invalid uses default", raw: "bad", want: defaultDNSCacheTTL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseDNSCacheTTL(tt.raw); got != tt.want {
				t.Fatalf("parseDNSCacheTTL(%q) = %s, want %s", tt.raw, got, tt.want)
			}
		})
	}
}

func TestTargetCacheKeyNormalizesHost(t *testing.T) {
	got := targetCacheKey(&target{Scheme: "https", Domain: "Emby.Example.Com.", Port: 443})
	want := "https://emby.example.com:443"
	if got != want {
		t.Fatalf("targetCacheKey() = %q, want %q", got, want)
	}
}

func TestCloneResolvedTargetCopiesAddresses(t *testing.T) {
	original := &resolvedTarget{dialAddrs: []string{net.JoinHostPort("203.0.113.10", "443")}}
	cloned := cloneResolvedTarget(original)
	if cloned == original {
		t.Fatal("cloneResolvedTarget returned original pointer")
	}
	cloned.dialAddrs[0] = net.JoinHostPort("203.0.113.11", "443")
	if original.dialAddrs[0] == cloned.dialAddrs[0] {
		t.Fatal("cloneResolvedTarget did not copy dialAddrs")
	}
}
