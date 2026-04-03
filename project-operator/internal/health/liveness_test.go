package health

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func newCheckerWithServer(t *testing.T, handler http.Handler, timeout time.Duration) *APIServerLivenessChecker {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	clientset, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatalf("creating clientset: %v", err)
	}
	return &APIServerLivenessChecker{
		client:  clientset,
		timeout: timeout,
	}
}

func TestAPIServerLivenessChecker_ImplementsChecker(t *testing.T) {
	checker := &APIServerLivenessChecker{}
	var fn func(*http.Request) error = checker.Check
	if fn == nil {
		t.Fatal("Check method should not be nil")
	}
}

func TestAPIServerLivenessChecker_Reachable(t *testing.T) {
	checker := newCheckerWithServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}), 5*time.Second)

	err := checker.Check(nil)
	if err != nil {
		t.Fatalf("expected nil error for reachable API server, got: %v", err)
	}
}

func TestAPIServerLivenessChecker_Unreachable(t *testing.T) {
	// Create a checker pointing to a closed server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()

	clientset, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatalf("creating clientset: %v", err)
	}
	checker := &APIServerLivenessChecker{
		client:  clientset,
		timeout: 1 * time.Second,
	}

	err = checker.Check(nil)
	if err == nil {
		t.Fatal("expected error for unreachable API server, got nil")
	}
}

func TestAPIServerLivenessChecker_Timeout(t *testing.T) {
	checker := newCheckerWithServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delay longer than checker timeout
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}), 100*time.Millisecond)

	err := checker.Check(nil)
	if err == nil {
		t.Fatal("expected error for timeout, got nil")
	}
}

func TestAPIServerLivenessChecker_UnhealthyResponse(t *testing.T) {
	checker := newCheckerWithServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("not ok"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}), 5*time.Second)

	err := checker.Check(nil)
	if err == nil {
		t.Fatal("expected error for unhealthy response, got nil")
	}
}

func TestNewAPIServerLivenessChecker_DefaultTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(server.Close)

	checker, err := NewAPIServerLivenessChecker(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if checker.timeout != defaultTimeout {
		t.Fatalf("expected default timeout %v, got %v", defaultTimeout, checker.timeout)
	}
}
