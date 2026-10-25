package collector

import (
	"context"
	"encoding/json"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/danaschwanden/agent-substrate-dashboard/backend/internal/httpapi"
	ateapi "go.ate.dev/substrate/api/ateapi/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type fakeControlServer struct {
	ateapi.UnimplementedControlServer
}

func (fakeControlServer) ListActors(_ context.Context, request *ateapi.ListActorsRequest) (*ateapi.ListActorsResponse, error) {
	states := []ateapi.ActorState{
		ateapi.ActorState_ACTOR_STATE_RESUMING,
		ateapi.ActorState_ACTOR_STATE_RUNNING,
		ateapi.ActorState_ACTOR_STATE_SUSPENDING,
		ateapi.ActorState_ACTOR_STATE_SUSPENDED,
		ateapi.ActorState_ACTOR_STATE_CRASHED,
		ateapi.ActorState_ACTOR_STATE_DELETING,
		ateapi.ActorState_ACTOR_STATE_REVERTING,
		ateapi.ActorState_ACTOR_STATE_UNSPECIFIED,
	}
	actors := make([]*ateapi.Actor, 0, len(states))
	for i, state := range states {
		actor := &ateapi.Actor{
			Metadata: &ateapi.ResourceMetadata{Atespace: request.GetAtespace(), Name: "actor-" + string(rune('a'+i)), Uid: "sensitive-actor-uid"},
			Status:   &ateapi.ActorStatus{State: state},
		}
		if i == 1 {
			actor.Status.WorkerAssignment = &ateapi.WorkerAssignment{Worker: &ateapi.ObjectRef{Name: "worker-a"}}
			actor.Status.AssignedNode = "node-a"
		}
		if state == ateapi.ActorState_ACTOR_STATE_CRASHED {
			actor.Status.Crash = &ateapi.ActorCrash{Message: "boom"}
		}
		actors = append(actors, actor)
	}
	if request.GetOptions().GetPageToken() == "" {
		return &ateapi.ListActorsResponse{Actors: actors[:4], NextPageToken: "actors-2"}, nil
	}
	return &ateapi.ListActorsResponse{Actors: append(actors[4:], &ateapi.Actor{
		Metadata: &ateapi.ResourceMetadata{Atespace: "other", Name: "out-of-scope"},
	})}, nil
}

func (fakeControlServer) ListWorkers(_ context.Context, request *ateapi.ListWorkersRequest) (*ateapi.ListWorkersResponse, error) {
	workers := []*ateapi.Worker{
		worker("worker-a", "pool-a", ateapi.WorkerState_WORKER_STATE_ACTIVE),
		worker("worker-b", "pool-a", ateapi.WorkerState_WORKER_STATE_DRAINING),
		worker("worker-c", "pool-a", ateapi.WorkerState_WORKER_STATE_UNAVAILABLE),
		worker("worker-d", "pool-a", ateapi.WorkerState_WORKER_STATE_UNSPECIFIED),
		worker("worker-other", "pool-b", ateapi.WorkerState_WORKER_STATE_ACTIVE),
	}
	if request.GetOptions().GetPageToken() == "" {
		return &ateapi.ListWorkersResponse{Workers: workers[:2], NextPageToken: "workers-2"}, nil
	}
	return &ateapi.ListWorkersResponse{Workers: workers[2:]}, nil
}

func (fakeControlServer) ListWorkerActorAssignments(_ context.Context, request *ateapi.ListWorkerActorAssignmentsRequest) (*ateapi.ListWorkerActorAssignmentsResponse, error) {
	if request.GetWorker().GetName() != "worker-a" {
		return &ateapi.ListWorkerActorAssignmentsResponse{}, nil
	}
	if request.GetOptions().GetPageToken() == "" {
		return &ateapi.ListWorkerActorAssignmentsResponse{
			ActorAssignments: []*ateapi.ActorAssignment{assignment("space-a", "actor-a")},
			NextPageToken:    "assignments-2",
		}, nil
	}
	return &ateapi.ListWorkerActorAssignmentsResponse{ActorAssignments: []*ateapi.ActorAssignment{
		assignment("space-a", "actor-b"),
		assignment("space-a", "actor-b"),
		assignment("other", "out-of-scope"),
	}}, nil
}

func worker(name, pool string, state ateapi.WorkerState) *ateapi.Worker {
	return &ateapi.Worker{
		Metadata:        &ateapi.ResourceMetadata{Name: name, Uid: "sensitive-worker-uid"},
		WorkerNamespace: "workers",
		WorkerPool:      pool,
		WorkerPod:       name + "-pod",
		NodeName:        "node-1",
		Labels:          map[string]string{"private.example/detail": "must-not-leak"},
		Status: &ateapi.WorkerStatus{
			State:     state,
			Capacity:  &ateapi.WorkerResources{CpuMillicores: 2000, MemoryBytes: 4096, Actors: 4},
			Allocated: &ateapi.WorkerResources{CpuMillicores: 500, MemoryBytes: 1024, Actors: 2},
		},
	}
}

func assignment(atespace, name string) *ateapi.ActorAssignment {
	return &ateapi.ActorAssignment{Actor: &ateapi.ObjectRef{Atespace: atespace, Name: name}}
}

func TestCollectorPaginatesScopesNormalizesAndJoins(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	ateapi.RegisterControlServer(server, fakeControlServer{})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	connection, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })

	wantTime := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	collector, err := New(ateapi.NewControlClient(connection), Config{
		Atespace: "space-a", WorkerPool: "pool-a", PageSize: 2, Now: func() time.Time { return wantTime },
	})
	if err != nil {
		t.Fatal(err)
	}
	observation, err := collector.Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !observation.ObservedAt.Equal(wantTime) {
		t.Fatalf("ObservedAt = %v, want %v", observation.ObservedAt, wantTime)
	}
	wantActorStates := []httpapi.ActorState{
		httpapi.ActorStateRESUMING, httpapi.ActorStateRUNNING, httpapi.ActorStateSUSPENDING,
		httpapi.ActorStateSUSPENDED, httpapi.ActorStateCRASHED, httpapi.ActorStateDELETING,
		httpapi.ActorStateREVERTING, httpapi.ActorStateUNKNOWN,
	}
	gotActorStates := make([]httpapi.ActorState, len(observation.Actors))
	for i, actor := range observation.Actors {
		gotActorStates[i] = actor.State
	}
	if !reflect.DeepEqual(gotActorStates, wantActorStates) {
		t.Fatalf("actor states = %v, want %v", gotActorStates, wantActorStates)
	}
	if got := *observation.Actors[1].WorkerName; got != "worker-a" {
		t.Fatalf("actor-b worker = %q", got)
	}
	if got := *observation.Actors[4].CrashSummary; got != "boom" {
		t.Fatalf("crash summary = %q", got)
	}

	wantWorkerStates := []httpapi.WorkerState{
		httpapi.WorkerStateACTIVE, httpapi.WorkerStateDRAINING,
		httpapi.WorkerStateUNAVAILABLE, httpapi.WorkerStateUNKNOWN,
	}
	gotWorkerStates := make([]httpapi.WorkerState, len(observation.Workers))
	for i, item := range observation.Workers {
		gotWorkerStates[i] = item.State
	}
	if !reflect.DeepEqual(gotWorkerStates, wantWorkerStates) {
		t.Fatalf("worker states = %v, want %v", gotWorkerStates, wantWorkerStates)
	}
	if got, want := observation.Workers[0].ActorNames, []string{"actor-a", "actor-b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("worker-a actors = %v, want %v", got, want)
	}
	if got := observation.Workers[1].ActorNames; got == nil || len(got) != 0 {
		t.Fatalf("worker-b actors = %#v, want non-nil empty list", got)
	}
	if got := observation.Workers[0].Resources[0]; got.Capacity != "2000" || got.Allocated != "500" {
		t.Fatalf("CPU resources = %+v", got)
	}
	serialized, err := json.Marshal(observation)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"sensitive-actor-uid", "sensitive-worker-uid", "must-not-leak", "private.example/detail"} {
		if strings.Contains(string(serialized), forbidden) {
			t.Fatalf("raw Control API field %q leaked through projection: %s", forbidden, serialized)
		}
	}
}
