// Package dashboard builds truthful, bounded snapshots from Control API observations.
package dashboard

import (
	"context"
	"time"

	"github.com/danaschwanden/agent-substrate-dashboard/backend/internal/httpapi"
)

// Observation is one complete, scoped Control API collection before history is applied.
type Observation struct {
	ObservedAt time.Time
	Actors     []httpapi.Actor
	Workers    []httpapi.Worker
}

// Observer obtains one coherent best-effort fleet observation.
type Observer interface {
	Observe(context.Context) (Observation, error)
}

// Config controls the bounded, in-memory observation core.
type Config struct {
	Scope              httpapi.Scope
	ComponentVersion   string
	ActorLimit         int
	WorkerLimit        int
	TransitionLimit    int
	OperationLimit     int
	Capabilities       []httpapi.Capability
	DensityWindow      time.Duration
	PollInterval       time.Duration
	ObservationTimeout time.Duration
	OnObservationError func(error)
}

func (c Config) withDefaults() Config {
	if c.ComponentVersion == "" {
		c.ComponentVersion = "development"
	}
	if c.ActorLimit <= 0 {
		c.ActorLimit = 500
	}
	if c.WorkerLimit <= 0 {
		c.WorkerLimit = 100
	}
	if c.TransitionLimit <= 0 {
		c.TransitionLimit = 500
	}
	if c.OperationLimit <= 0 {
		c.OperationLimit = 100
	}
	if len(c.Capabilities) == 0 {
		c.Capabilities = []httpapi.Capability{httpapi.CapabilityOBSERVE}
	} else {
		c.Capabilities = append([]httpapi.Capability(nil), c.Capabilities...)
	}
	if c.DensityWindow <= 0 {
		c.DensityWindow = 15 * time.Minute
	}
	if c.PollInterval <= 0 {
		c.PollInterval = 2 * time.Second
	}
	if c.ObservationTimeout <= 0 {
		c.ObservationTimeout = 10 * time.Second
	}
	return c
}
