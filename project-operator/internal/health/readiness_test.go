package health

import (
	"context"
	"net/http"
	"testing"
)

type fakeCacheSyncer struct {
	synced bool
}

func (f *fakeCacheSyncer) WaitForCacheSync(_ context.Context) bool {
	return f.synced
}

func TestCacheReadinessChecker_ImplementsChecker(t *testing.T) {
	checker := NewCacheReadinessChecker(&fakeCacheSyncer{synced: true})
	// healthz.Checker is func(*http.Request) error — verify the method exists
	var fn func(*http.Request) error = checker.Check
	if fn == nil {
		t.Fatal("Check method should not be nil")
	}
}

func TestCacheReadinessChecker_Synced(t *testing.T) {
	checker := NewCacheReadinessChecker(&fakeCacheSyncer{synced: true})
	err := checker.Check(nil)
	if err != nil {
		t.Fatalf("expected nil error for synced cache, got: %v", err)
	}
}

func TestCacheReadinessChecker_NotSynced(t *testing.T) {
	checker := NewCacheReadinessChecker(&fakeCacheSyncer{synced: false})
	err := checker.Check(nil)
	if err == nil {
		t.Fatal("expected error for unsynced cache, got nil")
	}
	expected := "informer caches not synced"
	if err.Error() != expected {
		t.Fatalf("expected error %q, got %q", expected, err.Error())
	}
}
