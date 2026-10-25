// Package config loads and validates dashboard backend process configuration.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/agent-substrate/agent-substrate-dashboard/backend/internal/control"
	"github.com/agent-substrate/agent-substrate-dashboard/backend/internal/httpapi"
)

const (
	defaultControlEndpoint   = "api.ate-system.svc:443"
	defaultControlServerName = "api.ate-system.svc"
)

// Runtime is the fully validated backend configuration.
type Runtime struct {
	ListenAddress        string
	Control              control.DialConfig
	Scope                httpapi.Scope
	ComponentVersion     string
	PageSize             int32
	ActorLimit           int
	WorkerLimit          int
	TransitionLimit      int
	OperationLimit       int
	EnableOperations     bool
	OperationConcurrency int
	OperationTimeout     time.Duration
	PollInterval         time.Duration
	ObservationTimeout   time.Duration
	DensityWindow        time.Duration
}

// Load reads process environment variables and fails on missing or malformed
// values. The token and trust bundle paths are deliberately explicit so a
// deployment cannot silently fall back to ambient Kubernetes credentials.
func Load(buildVersion string) (Runtime, error) {
	return load(os.LookupEnv, buildVersion)
}

type lookupEnv func(string) (string, bool)

func load(lookup lookupEnv, buildVersion string) (Runtime, error) {
	atespace, err := required(lookup, "DASHBOARD_ATESPACE")
	if err != nil {
		return Runtime{}, err
	}
	workerPool, err := required(lookup, "DASHBOARD_WORKER_POOL")
	if err != nil {
		return Runtime{}, err
	}
	trustBundle, err := required(lookup, "DASHBOARD_CONTROL_TRUST_BUNDLE_FILE")
	if err != nil {
		return Runtime{}, err
	}
	token, err := required(lookup, "DASHBOARD_CONTROL_TOKEN_FILE")
	if err != nil {
		return Runtime{}, err
	}

	pageSize, err := integer(lookup, "DASHBOARD_PAGE_SIZE", 500, 1, 1000)
	if err != nil {
		return Runtime{}, err
	}
	actorLimit, err := integer(lookup, "DASHBOARD_ACTOR_LIMIT", 500, 1, 100000)
	if err != nil {
		return Runtime{}, err
	}
	workerLimit, err := integer(lookup, "DASHBOARD_WORKER_LIMIT", 100, 1, 100000)
	if err != nil {
		return Runtime{}, err
	}
	transitionLimit, err := integer(lookup, "DASHBOARD_TRANSITION_LIMIT", 500, 1, 100000)
	if err != nil {
		return Runtime{}, err
	}
	operationLimit, err := integer(lookup, "DASHBOARD_OPERATION_LIMIT", 100, 1, 100000)
	if err != nil {
		return Runtime{}, err
	}
	enableOperations, err := boolean(lookup, "DASHBOARD_ENABLE_OPERATIONS", false)
	if err != nil {
		return Runtime{}, err
	}
	operationConcurrency, err := integer(lookup, "DASHBOARD_OPERATION_CONCURRENCY", 4, 1, 64)
	if err != nil {
		return Runtime{}, err
	}
	operationTimeout, err := duration(lookup, "DASHBOARD_OPERATION_TIMEOUT", 10*time.Second)
	if err != nil {
		return Runtime{}, err
	}
	pollInterval, err := duration(lookup, "DASHBOARD_POLL_INTERVAL", 2*time.Second)
	if err != nil {
		return Runtime{}, err
	}
	observationTimeout, err := duration(lookup, "DASHBOARD_OBSERVATION_TIMEOUT", 10*time.Second)
	if err != nil {
		return Runtime{}, err
	}
	densityWindow, err := duration(lookup, "DASHBOARD_DENSITY_WINDOW", 15*time.Minute)
	if err != nil {
		return Runtime{}, err
	}
	componentVersion := value(lookup, "DASHBOARD_COMPONENT_VERSION", buildVersion)
	if componentVersion == "" {
		componentVersion = "development"
	}
	return Runtime{
		ListenAddress: value(lookup, "DASHBOARD_LISTEN_ADDRESS", ":8080"),
		Control: control.DialConfig{
			Endpoint:        value(lookup, "DASHBOARD_CONTROL_ENDPOINT", defaultControlEndpoint),
			ServerName:      value(lookup, "DASHBOARD_CONTROL_SERVER_NAME", defaultControlServerName),
			TrustBundleFile: trustBundle,
			TokenFile:       token,
		},
		Scope:                httpapi.Scope{Atespace: atespace, WorkerPool: workerPool},
		ComponentVersion:     componentVersion,
		PageSize:             int32(pageSize),
		ActorLimit:           actorLimit,
		WorkerLimit:          workerLimit,
		TransitionLimit:      transitionLimit,
		OperationLimit:       operationLimit,
		EnableOperations:     enableOperations,
		OperationConcurrency: operationConcurrency,
		OperationTimeout:     operationTimeout,
		PollInterval:         pollInterval,
		ObservationTimeout:   observationTimeout,
		DensityWindow:        densityWindow,
	}, nil
}

func boolean(lookup lookupEnv, name string, fallback bool) (bool, error) {
	raw, ok := lookup(name)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", name)
	}
	return parsed, nil
}

func required(lookup lookupEnv, name string) (string, error) {
	value, ok := lookup(name)
	value = strings.TrimSpace(value)
	if !ok || value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}

func value(lookup lookupEnv, name, fallback string) string {
	if value, ok := lookup(name); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func integer(lookup lookupEnv, name string, fallback, minimum, maximum int) (int, error) {
	raw, ok := lookup(name)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || parsed < minimum || parsed > maximum {
		return 0, fmt.Errorf("%s must be an integer from %d through %d", name, minimum, maximum)
	}
	return parsed, nil
}

func duration(lookup lookupEnv, name string, fallback time.Duration) (time.Duration, error) {
	raw, ok := lookup(name)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive Go duration", name)
	}
	return parsed, nil
}
