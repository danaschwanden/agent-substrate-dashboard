package dashboard

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/danaschwanden/agent-substrate-dashboard/backend/internal/control"
	"github.com/danaschwanden/agent-substrate-dashboard/backend/internal/httpapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const maxConnectionMessage = 256

type densitySample struct {
	at       time.Time
	actors   int
	occupied int
}

// Core owns the ephemeral history and latest presentation snapshot. It is
// safe for concurrent polling and HTTP reads.
type Core struct {
	observer Observer
	config   Config
	now      func() time.Time
	started  time.Time

	mu          sync.RWMutex
	latest      httpapi.DashboardSnapshot
	hasSuccess  bool
	states      map[string]httpapi.ActorState
	transitions []httpapi.ObservedTransition
	operations  []httpapi.DashboardOperation
	samples     []densitySample
}

// New constructs a dashboard core with explicitly configured capabilities.
func New(observer Observer, config Config, now func() time.Time) *Core {
	config = config.withDefaults()
	if now == nil {
		now = time.Now
	}
	started := now().UTC()
	c := &Core{
		observer: observer,
		config:   config,
		now:      now,
		started:  started,
		states:   map[string]httpapi.ActorState{},
	}
	c.latest = c.emptySnapshot(started)
	return c
}

// Poll performs one observation. Failed polls preserve the last successful
// resource data and label only the Control API connection as stale.
func (c *Core) Poll(ctx context.Context) error {
	if c.observer == nil {
		return errors.New("dashboard observer is required")
	}
	observation, err := c.observer.Observe(ctx)
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		c.recordFailure(err)
		return err
	}
	c.recordSuccess(observation)
	return nil
}

// Run polls immediately, then waits one full interval after each completed
// request. This construction cannot overlap observations.
func (c *Core) Run(ctx context.Context) error {
	if c.observer == nil {
		return errors.New("dashboard observer is required")
	}
	for {
		pollContext, cancel := context.WithTimeout(ctx, c.config.ObservationTimeout)
		err := c.Poll(pollContext)
		cancel()
		if err != nil && ctx.Err() == nil && c.config.OnObservationError != nil {
			c.config.OnObservationError(err)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		timer := time.NewTimer(c.config.PollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// RecordOperation retains a completed dashboard-originated operation. Phase 3
// is observe-only, but the bounded seam is ready for Phase 5 to call.
func (c *Core) RecordOperation(operation httpapi.DashboardOperation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.operations = append([]httpapi.DashboardOperation{cloneOperation(operation)}, c.operations...)
	if len(c.operations) > c.config.OperationLimit {
		c.operations = c.operations[:c.config.OperationLimit]
	}
	c.latest.Operations = cloneOperations(c.operations)
}

// Snapshot returns a copy of the current bounded snapshot. Sample age is
// refreshed at read time even between polls.
func (c *Core) Snapshot() httpapi.DashboardSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := cloneSnapshot(c.latest)
	result.Efficiency.ObservedDensity = density(c.samples, c.config.DensityWindow, c.now().UTC())
	return result
}

// Ready reports whether at least one complete scoped observation succeeded.
func (c *Core) Ready() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hasSuccess
}

func (c *Core) recordFailure(err error) {
	state := httpapi.ConnectionStateUNAVAILABLE
	if c.hasSuccess {
		state = httpapi.ConnectionStateSTALE
	}
	message := safeObservationError(err)
	c.latest.Connection.State = state
	c.latest.Connection.Message = message
}

func safeObservationError(err error) string {
	switch status.Code(err) {
	case codes.PermissionDenied:
		return "Control API denied permission for the configured scope"
	case codes.Unauthenticated:
		return "Control API authentication failed"
	case codes.Unavailable:
		return "Control API is unavailable"
	case codes.DeadlineExceeded:
		return "Control API observation timed out"
	case codes.Canceled:
		return "Control API observation was canceled"
	default:
		return boundedMessage("Control API observation failed (" + status.Code(err).String() + ")")
	}
}

func (c *Core) recordSuccess(observation Observation) {
	observedAt := observation.ObservedAt.UTC()
	if observedAt.IsZero() {
		observedAt = c.now().UTC()
	}
	newStates := make(map[string]httpapi.ActorState, len(observation.Actors))
	if c.hasSuccess {
		for _, actor := range observation.Actors {
			newStates[actor.Name] = actor.State
			if previous, ok := c.states[actor.Name]; ok && previous != actor.State {
				c.transitions = append([]httpapi.ObservedTransition{{
					ObservedAt:    observedAt,
					ActorName:     actor.Name,
					PreviousState: previous,
					CurrentState:  actor.State,
				}}, c.transitions...)
			}
		}
		if len(c.transitions) > c.config.TransitionLimit {
			c.transitions = c.transitions[:c.config.TransitionLimit]
		}
	} else {
		for _, actor := range observation.Actors {
			newStates[actor.Name] = actor.State
		}
	}
	c.states = newStates

	actors := append([]httpapi.Actor(nil), observation.Actors...)
	workers := append([]httpapi.Worker(nil), observation.Workers...)
	sort.Slice(actors, func(i, j int) bool { return actors[i].Name < actors[j].Name })
	sort.Slice(workers, func(i, j int) bool { return workers[i].Name < workers[j].Name })
	actorSummary := summarizeActors(actors)
	workerSummary := summarizeWorkers(workers)
	c.samples = append(c.samples, densitySample{at: observedAt, actors: len(actors), occupied: workerSummary.Occupied})
	c.samples = trimSamples(c.samples, observedAt.Add(-c.config.DensityWindow))

	lastSuccess := observedAt
	c.latest = httpapi.DashboardSnapshot{
		ApiVersion:                httpapi.V1,
		ComponentVersion:          c.config.ComponentVersion,
		SupportedSubstrateVersion: control.SupportedSubstrateVersion,
		BackendStartedAt:          c.started,
		ObservedAt:                observedAt,
		Scope:                     c.config.Scope,
		Capabilities:              append([]httpapi.Capability(nil), c.config.Capabilities...),
		Connection: httpapi.ControlApiConnection{
			State:                       httpapi.ConnectionStateCONNECTED,
			Message:                     "",
			LastSuccessfulObservationAt: &lastSuccess,
		},
		ActorSummary:      actorSummary,
		WorkerSummary:     workerSummary,
		TransitionSummary: summarizeTransitions(c.transitions),
		Efficiency: httpapi.Efficiency{
			FleetRatio:      ratio(len(actors), len(workers)),
			ObservedDensity: density(c.samples, c.config.DensityWindow, observedAt),
		},
		Truncation: httpapi.Truncation{
			Actors:  truncation(len(actors), c.config.ActorLimit),
			Workers: truncation(len(workers), c.config.WorkerLimit),
		},
		Actors:      limited(actors, c.config.ActorLimit),
		Workers:     limited(workers, c.config.WorkerLimit),
		Transitions: append([]httpapi.ObservedTransition(nil), c.transitions...),
		Operations:  cloneOperations(c.operations),
	}
	c.hasSuccess = true
}

func (c *Core) emptySnapshot(at time.Time) httpapi.DashboardSnapshot {
	return httpapi.DashboardSnapshot{
		ApiVersion:                httpapi.V1,
		ComponentVersion:          c.config.ComponentVersion,
		SupportedSubstrateVersion: control.SupportedSubstrateVersion,
		BackendStartedAt:          c.started,
		ObservedAt:                at,
		Scope:                     c.config.Scope,
		Capabilities:              append([]httpapi.Capability(nil), c.config.Capabilities...),
		Connection:                httpapi.ControlApiConnection{State: httpapi.ConnectionStateUNAVAILABLE, Message: "Awaiting first Control API observation"},
		ActorSummary:              httpapi.ActorSummary{States: []httpapi.ActorStateCount{}},
		WorkerSummary:             httpapi.WorkerSummary{States: []httpapi.WorkerStateCount{}},
		TransitionSummary:         httpapi.TransitionSummary{},
		Efficiency:                httpapi.Efficiency{FleetRatio: ratio(0, 0), ObservedDensity: density(nil, c.config.DensityWindow, at)},
		Truncation:                httpapi.Truncation{Actors: truncation(0, c.config.ActorLimit), Workers: truncation(0, c.config.WorkerLimit)},
		Actors:                    []httpapi.Actor{},
		Workers:                   []httpapi.Worker{},
		Transitions:               []httpapi.ObservedTransition{},
		Operations:                []httpapi.DashboardOperation{},
	}
}

func summarizeActors(actors []httpapi.Actor) httpapi.ActorSummary {
	counts := map[httpapi.ActorState]int{}
	for _, actor := range actors {
		counts[actor.State]++
	}
	states := make([]httpapi.ActorStateCount, 0, len(counts))
	for _, state := range actorStateOrder {
		if count := counts[state]; count > 0 {
			states = append(states, httpapi.ActorStateCount{State: state, Count: count})
		}
	}
	return httpapi.ActorSummary{Total: len(actors), States: states}
}

func summarizeWorkers(workers []httpapi.Worker) httpapi.WorkerSummary {
	counts := map[httpapi.WorkerState]int{}
	occupied, assignments := 0, 0
	for _, worker := range workers {
		counts[worker.State]++
		assignments += len(worker.ActorNames)
		if len(worker.ActorNames) > 0 {
			occupied++
		}
	}
	states := make([]httpapi.WorkerStateCount, 0, len(counts))
	for _, state := range workerStateOrder {
		if count := counts[state]; count > 0 {
			states = append(states, httpapi.WorkerStateCount{State: state, Count: count})
		}
	}
	return httpapi.WorkerSummary{Total: len(workers), Occupied: occupied, Assignments: assignments, States: states}
}

func summarizeTransitions(transitions []httpapi.ObservedTransition) httpapi.TransitionSummary {
	result := httpapi.TransitionSummary{Total: len(transitions)}
	for _, transition := range transitions {
		if transition.PreviousState == httpapi.ActorStateSUSPENDED &&
			(transition.CurrentState == httpapi.ActorStateRESUMING || transition.CurrentState == httpapi.ActorStateRUNNING) {
			result.Resumes++
		}
		if transition.PreviousState == httpapi.ActorStateRUNNING &&
			(transition.CurrentState == httpapi.ActorStateSUSPENDING || transition.CurrentState == httpapi.ActorStateSUSPENDED) {
			result.Suspends++
		}
	}
	return result
}

func ratio(numerator, denominator int) httpapi.Ratio {
	result := httpapi.Ratio{Numerator: numerator, Denominator: denominator}
	if denominator > 0 {
		value := float64(numerator) / float64(denominator)
		result.Value = &value
	}
	return result
}

func density(samples []densitySample, window time.Duration, now time.Time) httpapi.ObservedDensity {
	result := httpapi.ObservedDensity{WindowSeconds: int(window.Seconds())}
	cutoff := now.Add(-window)
	retained := make([]densitySample, 0, len(samples))
	for _, sample := range samples {
		if !sample.at.Before(cutoff) {
			retained = append(retained, sample)
		}
	}
	if len(retained) == 0 {
		return result
	}
	oldest := retained[0].at
	peak := retained[0]
	for _, sample := range retained[1:] {
		if sample.at.Before(oldest) {
			oldest = sample.at
		}
		if sample.occupied > peak.occupied || (sample.occupied == peak.occupied && sample.actors > peak.actors) {
			peak = sample
		}
	}
	age := now.Sub(oldest)
	if age < 0 {
		age = 0
	}
	result.SampleAgeSeconds = int(age.Seconds())
	result.ActorsAtPeak = peak.actors
	result.OccupiedWorkersAtPeak = peak.occupied
	if peak.occupied > 0 {
		value := float64(peak.actors) / float64(peak.occupied)
		result.Available = true
		result.Value = &value
	}
	return result
}

func trimSamples(samples []densitySample, cutoff time.Time) []densitySample {
	first := 0
	for first < len(samples) && samples[first].at.Before(cutoff) {
		first++
	}
	return append([]densitySample(nil), samples[first:]...)
}

func truncation(total, limit int) httpapi.TruncationDetail {
	returned := total
	if returned > limit {
		returned = limit
	}
	return httpapi.TruncationDetail{Total: total, Returned: returned, Limit: limit, Truncated: returned < total}
}

func limited[T any](values []T, limit int) []T {
	if len(values) > limit {
		values = values[:limit]
	}
	return append([]T(nil), values...)
}

func boundedMessage(message string) string {
	message = strings.Join(strings.Fields(message), " ")
	runes := []rune(message)
	if len(runes) > maxConnectionMessage {
		return string(runes[:maxConnectionMessage])
	}
	return message
}

func cloneSnapshot(source httpapi.DashboardSnapshot) httpapi.DashboardSnapshot {
	result := source
	result.Capabilities = append([]httpapi.Capability(nil), source.Capabilities...)
	result.ActorSummary.States = append([]httpapi.ActorStateCount(nil), source.ActorSummary.States...)
	result.WorkerSummary.States = append([]httpapi.WorkerStateCount(nil), source.WorkerSummary.States...)
	result.Actors = append([]httpapi.Actor(nil), source.Actors...)
	result.Workers = make([]httpapi.Worker, len(source.Workers))
	for i, worker := range source.Workers {
		result.Workers[i] = worker
		result.Workers[i].ActorNames = append([]string(nil), worker.ActorNames...)
		result.Workers[i].Resources = append([]httpapi.ResourceAllocation(nil), worker.Resources...)
	}
	result.Transitions = append([]httpapi.ObservedTransition(nil), source.Transitions...)
	result.Operations = cloneOperations(source.Operations)
	return result
}

func cloneOperations(source []httpapi.DashboardOperation) []httpapi.DashboardOperation {
	result := make([]httpapi.DashboardOperation, len(source))
	for i, operation := range source {
		result[i] = cloneOperation(operation)
	}
	return result
}

func cloneOperation(source httpapi.DashboardOperation) httpapi.DashboardOperation {
	result := source
	result.Results = append([]httpapi.ActorOperationResult(nil), source.Results...)
	return result
}

var actorStateOrder = []httpapi.ActorState{
	httpapi.ActorStateRESUMING, httpapi.ActorStateRUNNING, httpapi.ActorStateSUSPENDING,
	httpapi.ActorStateSUSPENDED, httpapi.ActorStateCRASHED, httpapi.ActorStateDELETING,
	httpapi.ActorStateREVERTING, httpapi.ActorStateUNKNOWN,
}

var workerStateOrder = []httpapi.WorkerState{
	httpapi.WorkerStateACTIVE, httpapi.WorkerStateDRAINING,
	httpapi.WorkerStateUNAVAILABLE, httpapi.WorkerStateUNKNOWN,
}
