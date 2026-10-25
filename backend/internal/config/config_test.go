package config

import (
	"strings"
	"testing"
	"time"
)

func environment(values map[string]string) lookupEnv {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}

func validEnvironment() map[string]string {
	return map[string]string{
		"DASHBOARD_ATESPACE":                  "agents",
		"DASHBOARD_WORKER_POOL":               "default",
		"DASHBOARD_CONTROL_TRUST_BUNDLE_FILE": "/identity/trust-bundle.pem",
		"DASHBOARD_CONTROL_TOKEN_FILE":        "/identity/token",
	}
}

func TestLoadDefaultsToObserveOnlyRuntime(t *testing.T) {
	config, err := load(environment(validEnvironment()), "v0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if config.Control.Endpoint != "api.ate-system.svc:443" || config.Control.ServerName != "api.ate-system.svc" {
		t.Fatalf("Control defaults = %+v", config.Control)
	}
	if config.ComponentVersion != "v0.1.0" || config.ListenAddress != ":8080" {
		t.Fatalf("process defaults = %+v", config)
	}
	if config.PollInterval != 2*time.Second || config.ObservationTimeout != 10*time.Second || config.DensityWindow != 15*time.Minute {
		t.Fatalf("duration defaults = %+v", config)
	}
	if config.EnableOperations || config.OperationConcurrency != 4 || config.OperationTimeout != 10*time.Second {
		t.Fatalf("operation defaults = %+v", config)
	}
}

func TestLoadExplicitlyEnablesOperations(t *testing.T) {
	values := validEnvironment()
	values["DASHBOARD_ENABLE_OPERATIONS"] = "true"
	values["DASHBOARD_OPERATION_CONCURRENCY"] = "7"
	values["DASHBOARD_OPERATION_TIMEOUT"] = "12s"
	config, err := load(environment(values), "test")
	if err != nil {
		t.Fatal(err)
	}
	if !config.EnableOperations || config.OperationConcurrency != 7 || config.OperationTimeout != 12*time.Second {
		t.Fatalf("operation config = %+v", config)
	}
}

func TestLoadRejectsMissingAndMalformedConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		change func(map[string]string)
		want   string
	}{
		{name: "missing atespace", change: func(values map[string]string) { delete(values, "DASHBOARD_ATESPACE") }, want: "DASHBOARD_ATESPACE is required"},
		{name: "missing trust", change: func(values map[string]string) { delete(values, "DASHBOARD_CONTROL_TRUST_BUNDLE_FILE") }, want: "DASHBOARD_CONTROL_TRUST_BUNDLE_FILE is required"},
		{name: "invalid limit", change: func(values map[string]string) { values["DASHBOARD_ACTOR_LIMIT"] = "0" }, want: "DASHBOARD_ACTOR_LIMIT must be"},
		{name: "invalid duration", change: func(values map[string]string) { values["DASHBOARD_POLL_INTERVAL"] = "often" }, want: "DASHBOARD_POLL_INTERVAL must be"},
		{name: "invalid operation switch", change: func(values map[string]string) { values["DASHBOARD_ENABLE_OPERATIONS"] = "sometimes" }, want: "DASHBOARD_ENABLE_OPERATIONS must be true or false"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			values := validEnvironment()
			test.change(values)
			_, err := load(environment(values), "test")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("load error = %v, want containing %q", err, test.want)
			}
		})
	}
}
