package health

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const defaultTimeout = 5 * time.Second

// APIServerLivenessChecker verifies connectivity to the Kubernetes API server.
type APIServerLivenessChecker struct {
	client  kubernetes.Interface
	timeout time.Duration
}

// NewAPIServerLivenessChecker creates a liveness checker that probes API server connectivity.
// Uses a 5-second default timeout.
func NewAPIServerLivenessChecker(cfg *rest.Config) (*APIServerLivenessChecker, error) {
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes client: %w", err)
	}
	return &APIServerLivenessChecker{
		client:  clientset,
		timeout: defaultTimeout,
	}, nil
}

// NewAPIServerLivenessCheckerWithTimeout creates a liveness checker with a custom timeout.
func NewAPIServerLivenessCheckerWithTimeout(cfg *rest.Config, timeout time.Duration) (*APIServerLivenessChecker, error) {
	checker, err := NewAPIServerLivenessChecker(cfg)
	if err != nil {
		return nil, err
	}
	checker.timeout = timeout
	return checker, nil
}

// Check implements healthz.Checker. Returns nil when API server is reachable, error otherwise.
func (a *APIServerLivenessChecker) Check(_ *http.Request) error {
	ctx, cancel := context.WithTimeout(context.Background(), a.timeout)
	defer cancel()

	body, err := a.client.Discovery().RESTClient().Get().AbsPath("/healthz").DoRaw(ctx)
	if err != nil {
		return fmt.Errorf("API server unreachable: %w", err)
	}
	if string(body) != "ok" {
		return fmt.Errorf("API server unhealthy: %s", string(body))
	}
	return nil
}
