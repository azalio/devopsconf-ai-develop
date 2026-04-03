package health

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// CacheSyncer is the subset of cache.Cache needed for readiness checks.
type CacheSyncer interface {
	WaitForCacheSync(ctx context.Context) bool
}

// CacheReadinessChecker verifies that controller-runtime informer caches are synced.
type CacheReadinessChecker struct {
	cache CacheSyncer
}

// NewCacheReadinessChecker creates a readiness checker that verifies cache sync state.
// Accepts cache.Cache or any type implementing WaitForCacheSync.
func NewCacheReadinessChecker(c CacheSyncer) *CacheReadinessChecker {
	return &CacheReadinessChecker{cache: c}
}

// Check implements healthz.Checker. Returns nil when caches are synced, error otherwise.
func (c *CacheReadinessChecker) Check(_ *http.Request) error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if !c.cache.WaitForCacheSync(ctx) {
		return fmt.Errorf("informer caches not synced")
	}
	return nil
}
