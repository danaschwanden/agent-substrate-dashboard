package dashboard

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agent-substrate/agent-substrate-dashboard/backend/internal/httpapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type sequenceObserver struct {
	mu           sync.Mutex
	observations []Observation
	errors       []error
	calls        int
}

func (o *sequenceObserver) Observe(context.Context) (Observation, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	index := o.calls
	o.calls++
	var observation Observation
	var err error
	if index < len(o.observations) {
		observation = o.observations[index]
	}
	if index < len(o.errors) {
		err = o.errors[index]
	}
	return observation, err
}

func TestCoreMetricsTransitionsTruncationAndStaleRetention(t *testing.T) {
	t0 := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	now := t0
	observer := &sequenceObserver{
		observations: []Observation{
			{
				ObservedAt: t0,
				Actors: []httpapi.Actor{
					{Name: "a", State: httpapi.ActorStateSUSPENDED},
					{Name: "b", State: httpapi.ActorStateRUNNING},
					{Name: "c", State: httpapi.ActorStateUNKNOWN},
				},
				Workers: []httpapi.Worker{
					{Name: "w1", State: httpapi.WorkerStateACTIVE, ActorNames: []string{"b"}},
					{Name: "w2", State: httpapi.WorkerStateACTIVE, ActorNames: []string{}},
				},
			},
			{
				ObservedAt: t0.Add(time.Minute),
				Actors: []httpapi.Actor{
					{Name: "a", State: httpapi.ActorStateRUNNING},
					{Name: "b", State: httpapi.ActorStateSUSPENDED},
					{Name: "c", State: httpapi.ActorStateUNKNOWN},
					{Name: "d", State: httpapi.ActorStateRUNNING},
				},
				Workers: []httpapi.Worker{
					{Name: "w1", State: httpapi.WorkerStateACTIVE, ActorNames: []string{"a", "d"}},
					{Name: "w2", State: httpapi.WorkerStateACTIVE, ActorNames: []string{"b"}},
				},
			},
		},
		errors: []error{nil, nil, errors.New("permission denied for secret details")},
	}
	core := New(observer, Config{
		Scope:           httpapi.Scope{Atespace: "space", WorkerPool: "pool"},
		ActorLimit:      2,
		WorkerLimit:     1,
		TransitionLimit: 1,
		DensityWindow:   15 * time.Minute,
	}, func() time.Time { return now })

	if core.Ready() {
		t.Fatal("core is ready before its first observation")
	}
	if err := core.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	now = t0.Add(time.Minute)
	if err := core.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	snapshot := core.Snapshot()
	if !core.Ready() {
		t.Fatal("core is not ready after a successful observation")
	}
	if snapshot.ActorSummary.Total != 4 || len(snapshot.Actors) != 2 || !snapshot.Truncation.Actors.Truncated {
		t.Fatalf("actor truncation/summary = total %d, returned %d, detail %+v", snapshot.ActorSummary.Total, len(snapshot.Actors), snapshot.Truncation.Actors)
	}
	if snapshot.WorkerSummary.Total != 2 || snapshot.WorkerSummary.Occupied != 2 || snapshot.WorkerSummary.Assignments != 3 {
		t.Fatalf("worker summary = %+v", snapshot.WorkerSummary)
	}
	if got := snapshot.Efficiency.FleetRatio; got.Numerator != 4 || got.Denominator != 2 || got.Value == nil || *got.Value != 2 {
		t.Fatalf("fleet ratio = %+v", got)
	}
	if got := snapshot.Efficiency.ObservedDensity; !got.Available || got.ActorsAtPeak != 4 || got.OccupiedWorkersAtPeak != 2 || got.Value == nil || *got.Value != 2 || got.SampleAgeSeconds != 60 {
		t.Fatalf("observed density = %+v", got)
	}
	if len(snapshot.Transitions) != 1 || snapshot.Transitions[0].ActorName != "b" {
		t.Fatalf("bounded transitions = %+v", snapshot.Transitions)
	}
	if snapshot.TransitionSummary.Total != 1 || snapshot.TransitionSummary.Suspends != 1 {
		t.Fatalf("transition summary = %+v", snapshot.TransitionSummary)
	}

	now = t0.Add(2 * time.Minute)
	if err := core.Poll(context.Background()); err == nil {
		t.Fatal("failed poll unexpectedly succeeded")
	}
	stale := core.Snapshot()
	if stale.Connection.State != httpapi.ConnectionStateSTALE {
		t.Fatalf("connection state = %q, want STALE", stale.Connection.State)
	}
	if strings.Contains(stale.Connection.Message, "secret details") {
		t.Fatalf("upstream failure detail leaked into snapshot: %q", stale.Connection.Message)
	}
	if stale.ActorSummary.Total != 4 || !stale.ObservedAt.Equal(t0.Add(time.Minute)) {
		t.Fatalf("stale poll did not preserve last observation: %+v", stale)
	}
	if stale.Connection.LastSuccessfulObservationAt == nil || !stale.Connection.LastSuccessfulObservationAt.Equal(t0.Add(time.Minute)) {
		t.Fatalf("last successful observation = %v", stale.Connection.LastSuccessfulObservationAt)
	}
}

func TestDensityEdgeCases(t *testing.T) {
	t0 := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		samples   []densitySample
		now       time.Time
		available bool
		actors    int
		occupied  int
		value     float64
		age       int
	}{
		{name: "incomplete window with no workers", samples: []densitySample{{at: t0, actors: 10}}, now: t0, available: false, actors: 10},
		{name: "workers but none occupied", samples: []densitySample{{at: t0, actors: 10, occupied: 0}}, now: t0.Add(time.Minute), available: false, actors: 10, age: 60},
		{name: "changing actors chooses demand peak", samples: []densitySample{{at: t0, actors: 20, occupied: 2}, {at: t0.Add(time.Minute), actors: 12, occupied: 3}}, now: t0.Add(2 * time.Minute), available: true, actors: 12, occupied: 3, value: 4, age: 120},
		{name: "tie chooses higher actor count", samples: []densitySample{{at: t0, actors: 10, occupied: 2}, {at: t0.Add(time.Minute), actors: 14, occupied: 2}}, now: t0.Add(2 * time.Minute), available: true, actors: 14, occupied: 2, value: 7, age: 120},
		{name: "expired window is unavailable", samples: []densitySample{{at: t0, actors: 10, occupied: 2}}, now: t0.Add(16 * time.Minute), available: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := density(test.samples, 15*time.Minute, test.now)
			if got.Available != test.available || got.ActorsAtPeak != test.actors || got.OccupiedWorkersAtPeak != test.occupied || got.SampleAgeSeconds != test.age {
				t.Fatalf("density = %+v", got)
			}
			if test.available && (got.Value == nil || *got.Value != test.value) {
				t.Fatalf("density value = %v, want %v", got.Value, test.value)
			}
			if !test.available && got.Value != nil {
				t.Fatalf("unavailable density value = %v, want nil", *got.Value)
			}
		})
	}
}

func TestObservationFailuresAreClassifiedWithoutUpstreamDetails(t *testing.T) {
	tests := []struct {
		code codes.Code
		want string
	}{
		{code: codes.PermissionDenied, want: "Control API denied permission for the configured scope"},
		{code: codes.Unauthenticated, want: "Control API authentication failed"},
		{code: codes.Unavailable, want: "Control API is unavailable"},
		{code: codes.DeadlineExceeded, want: "Control API observation timed out"},
	}
	for _, test := range tests {
		errorWithSecret := status.Error(test.code, "Bearer secret-that-must-not-leak")
		if got := safeObservationError(errorWithSecret); got != test.want {
			t.Fatalf("safeObservationError(%s) = %q, want %q", test.code, got, test.want)
		}
	}
}

type blockingObserver struct {
	mu            sync.Mutex
	active        int
	maximumActive int
	calls         int
}

func (o *blockingObserver) Observe(ctx context.Context) (Observation, error) {
	o.mu.Lock()
	o.active++
	o.calls++
	if o.active > o.maximumActive {
		o.maximumActive = o.active
	}
	o.mu.Unlock()
	select {
	case <-time.After(15 * time.Millisecond):
	case <-ctx.Done():
		o.mu.Lock()
		o.active--
		o.mu.Unlock()
		return Observation{}, ctx.Err()
	}
	o.mu.Lock()
	o.active--
	o.mu.Unlock()
	return Observation{ObservedAt: time.Now()}, nil
}

func TestRunDoesNotOverlapAndCancels(t *testing.T) {
	observer := &blockingObserver{}
	core := New(observer, Config{PollInterval: time.Millisecond}, time.Now)
	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Millisecond)
	defer cancel()
	if err := core.Run(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run error = %v, want deadline exceeded", err)
	}
	observer.mu.Lock()
	defer observer.mu.Unlock()
	if observer.maximumActive != 1 {
		t.Fatalf("maximum concurrent observations = %d, want 1", observer.maximumActive)
	}
	if observer.calls < 2 {
		t.Fatalf("observation calls = %d, want at least 2", observer.calls)
	}
}

func TestOperationsAreBoundedAndCopied(t *testing.T) {
	core := New(&sequenceObserver{}, Config{OperationLimit: 1}, time.Now)
	first := httpapi.DashboardOperation{Id: "first", Results: []httpapi.ActorOperationResult{{ActorName: "a"}}}
	second := httpapi.DashboardOperation{Id: "second", Results: []httpapi.ActorOperationResult{{ActorName: "b"}}}
	core.RecordOperation(first)
	core.RecordOperation(second)
	snapshot := core.Snapshot()
	if len(snapshot.Operations) != 1 || snapshot.Operations[0].Id != "second" {
		t.Fatalf("operations = %+v", snapshot.Operations)
	}
	snapshot.Operations[0].Results[0].ActorName = "mutated"
	if got := core.Snapshot().Operations[0].Results[0].ActorName; got != "b" {
		t.Fatalf("stored operation was mutated through snapshot copy: %q", got)
	}
}

func TestConfiguredCapabilitiesAreAdvertisedAndCopied(t *testing.T) {
	capabilities := []httpapi.Capability{httpapi.CapabilityOBSERVE, httpapi.CapabilityRESUME, httpapi.CapabilitySUSPEND}
	core := New(&sequenceObserver{}, Config{Capabilities: capabilities}, time.Now)
	capabilities[1] = httpapi.CapabilityOBSERVE
	snapshot := core.Snapshot()
	if got := snapshot.Capabilities[1]; got != httpapi.CapabilityRESUME {
		t.Fatalf("capability = %q, want RESUME", got)
	}
	snapshot.Capabilities[1] = httpapi.CapabilityOBSERVE
	if got := core.Snapshot().Capabilities[1]; got != httpapi.CapabilityRESUME {
		t.Fatalf("stored capability mutated through snapshot copy: %q", got)
	}
}
