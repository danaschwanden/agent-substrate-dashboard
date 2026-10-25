package lifecycle

import (
	"context"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/agent-substrate/agent-substrate-dashboard/backend/internal/httpapi"
	ateapi "go.ate.dev/substrate/api/ateapi/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type fakeLifecycleServer struct {
	ateapi.UnimplementedControlServer
	mu        sync.Mutex
	active    int
	maxActive int
	refs      []*ateapi.ObjectRef
}

func (s *fakeLifecycleServer) ResumeActor(_ context.Context, request *ateapi.ResumeActorRequest) (*ateapi.ResumeActorResponse, error) {
	return s.respond(request.GetActor(), true)
}

func (s *fakeLifecycleServer) SuspendActor(_ context.Context, request *ateapi.SuspendActorRequest) (*ateapi.SuspendActorResponse, error) {
	_, err := s.respond(request.GetActor(), false)
	return &ateapi.SuspendActorResponse{}, err
}

func (s *fakeLifecycleServer) respond(actor *ateapi.ObjectRef, resume bool) (*ateapi.ResumeActorResponse, error) {
	s.mu.Lock()
	s.active++
	if s.active > s.maxActive {
		s.maxActive = s.active
	}
	s.refs = append(s.refs, &ateapi.ObjectRef{Atespace: actor.GetAtespace(), Name: actor.GetName()})
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.active--
		s.mu.Unlock()
	}()
	time.Sleep(20 * time.Millisecond)
	if actor.GetName() == "conflict" {
		return nil, status.Error(codes.Aborted, "raw version conflict details")
	}
	if actor.GetName() == "denied" {
		return nil, status.Error(codes.PermissionDenied, "raw policy details")
	}
	return &ateapi.ResumeActorResponse{Resumed: resume}, nil
}

type recordingSink struct {
	operations []httpapi.DashboardOperation
}

func (r *recordingSink) RecordOperation(operation httpapi.DashboardOperation) {
	r.operations = append(r.operations, operation)
}

func TestExecutorScopesBoundsAndReportsNonAtomicResults(t *testing.T) {
	client, server := lifecycleTestClient(t)
	recorder := &recordingSink{}
	wantTime := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	executor, err := New(client, recorder, Config{
		Atespace: "space-a", Concurrency: 2, Timeout: time.Second,
		Now: func() time.Time { return wantTime }, NewID: func() string { return "op-test" },
	})
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"actor-a", "actor-b", "conflict", "denied"}
	operation := executor.Execute(context.Background(), httpapi.OperationTypeRESUME, names)

	if operation.Id != "op-test" || operation.Type != httpapi.OperationTypeRESUME || !operation.RequestedAt.Equal(wantTime) || !operation.CompletedAt.Equal(wantTime) {
		t.Fatalf("operation metadata = %+v", operation)
	}
	gotNames := make([]string, len(operation.Results))
	for i, result := range operation.Results {
		gotNames[i] = result.ActorName
	}
	if !reflect.DeepEqual(gotNames, names) {
		t.Fatalf("result order = %v, want %v", gotNames, names)
	}
	if operation.Results[0].Outcome != httpapi.SUCCEEDED || operation.Results[1].Outcome != httpapi.SUCCEEDED {
		t.Fatalf("successful results = %+v", operation.Results[:2])
	}
	if operation.Results[2].Outcome != httpapi.FAILED || operation.Results[2].Message != "actor state changed concurrently; retry if the requested transition is still needed" {
		t.Fatalf("conflict result = %+v", operation.Results[2])
	}
	if operation.Results[3].Outcome != httpapi.FAILED || operation.Results[3].Message != "Control API denied permission for this actor" {
		t.Fatalf("permission result = %+v", operation.Results[3])
	}
	if server.maxActive != 2 {
		t.Fatalf("maximum concurrent RPCs = %d, want 2", server.maxActive)
	}
	for _, ref := range server.refs {
		if ref.GetAtespace() != "space-a" {
			t.Fatalf("actor reference escaped configured scope: %+v", ref)
		}
	}
	if len(recorder.operations) != 1 || !reflect.DeepEqual(recorder.operations[0], operation) {
		t.Fatalf("recorded operations = %+v", recorder.operations)
	}
}

func TestExecutorCallsSuspendAndRetainsFailureAsResult(t *testing.T) {
	client, _ := lifecycleTestClient(t)
	recorder := &recordingSink{}
	executor, err := New(client, recorder, Config{Atespace: "space-a", Concurrency: 1, NewID: func() string { return "suspend-test" }})
	if err != nil {
		t.Fatal(err)
	}
	operation := executor.Execute(context.Background(), httpapi.OperationTypeSUSPEND, []string{"denied"})
	if operation.Type != httpapi.OperationTypeSUSPEND || operation.Results[0].Outcome != httpapi.FAILED {
		t.Fatalf("suspend operation = %+v", operation)
	}
	if len(recorder.operations) != 1 {
		t.Fatalf("recorded operations = %d, want 1", len(recorder.operations))
	}
}

func lifecycleTestClient(t *testing.T) (ateapi.ControlClient, *fakeLifecycleServer) {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	fake := &fakeLifecycleServer{}
	ateapi.RegisterControlServer(server, fake)
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
	return ateapi.NewControlClient(connection), fake
}
