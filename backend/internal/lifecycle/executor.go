// Package lifecycle executes explicitly targeted Agent Substrate lifecycle operations.
package lifecycle

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/agent-substrate/agent-substrate-dashboard/backend/internal/httpapi"
	ateapi "go.ate.dev/substrate/api/ateapi/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const maxOutcomeMessage = 256

// ControlClient is the lifecycle-only Control API surface used by the executor.
type ControlClient interface {
	ResumeActor(context.Context, *ateapi.ResumeActorRequest, ...grpc.CallOption) (*ateapi.ResumeActorResponse, error)
	SuspendActor(context.Context, *ateapi.SuspendActorRequest, ...grpc.CallOption) (*ateapi.SuspendActorResponse, error)
}

// Recorder retains completed dashboard-originated operations.
type Recorder interface {
	RecordOperation(httpapi.DashboardOperation)
}

// Config fixes every operation to one server-owned atespace and bounds its work.
type Config struct {
	Atespace    string
	Concurrency int
	Timeout     time.Duration
	Now         func() time.Time
	NewID       func() string
}

// Executor runs non-atomic bulk operations while preserving target order.
type Executor struct {
	client   ControlClient
	recorder Recorder
	config   Config
}

func New(client ControlClient, recorder Recorder, config Config) (*Executor, error) {
	if client == nil {
		return nil, fmt.Errorf("Control API client is required")
	}
	if recorder == nil {
		return nil, fmt.Errorf("operation recorder is required")
	}
	if config.Atespace == "" {
		return nil, fmt.Errorf("atespace is required")
	}
	if config.Concurrency <= 0 {
		config.Concurrency = 4
	}
	if config.Timeout <= 0 {
		config.Timeout = 10 * time.Second
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.NewID == nil {
		config.NewID = randomID
	}
	return &Executor{client: client, recorder: recorder, config: config}, nil
}

// Execute returns one outcome per explicit actor target and never rolls back successes.
func (e *Executor) Execute(ctx context.Context, operationType httpapi.OperationType, actorNames []string) httpapi.DashboardOperation {
	requestedAt := e.config.Now().UTC()
	operation := httpapi.DashboardOperation{
		Id:          e.config.NewID(),
		Type:        operationType,
		RequestedAt: requestedAt,
		Results:     make([]httpapi.ActorOperationResult, len(actorNames)),
	}

	operationContext, cancel := context.WithTimeout(ctx, e.config.Timeout)
	defer cancel()
	type job struct {
		index int
		name  string
	}
	jobs := make(chan job)
	done := make(chan struct{})
	workers := min(e.config.Concurrency, len(actorNames))
	for range workers {
		go func() {
			for target := range jobs {
				operation.Results[target.index] = e.executeOne(operationContext, operationType, target.name)
			}
			done <- struct{}{}
		}()
	}
	for index, name := range actorNames {
		jobs <- job{index: index, name: name}
	}
	close(jobs)
	for range workers {
		<-done
	}
	operation.CompletedAt = e.config.Now().UTC()
	e.recorder.RecordOperation(operation)
	return operation
}

func (e *Executor) executeOne(ctx context.Context, operationType httpapi.OperationType, actorName string) httpapi.ActorOperationResult {
	actor := &ateapi.ObjectRef{Atespace: e.config.Atespace, Name: actorName}
	var err error
	switch operationType {
	case httpapi.OperationTypeRESUME:
		_, err = e.client.ResumeActor(ctx, &ateapi.ResumeActorRequest{Actor: actor})
	case httpapi.OperationTypeSUSPEND:
		_, err = e.client.SuspendActor(ctx, &ateapi.SuspendActorRequest{Actor: actor})
	default:
		err = status.Error(codes.InvalidArgument, "unsupported lifecycle operation")
	}
	if err != nil {
		return httpapi.ActorOperationResult{ActorName: actorName, Outcome: httpapi.FAILED, Message: safeError(err)}
	}
	return httpapi.ActorOperationResult{ActorName: actorName, Outcome: httpapi.SUCCEEDED, Message: ""}
}

func safeError(err error) string {
	var message string
	switch status.Code(err) {
	case codes.Aborted, codes.FailedPrecondition:
		message = "actor state changed concurrently; retry if the requested transition is still needed"
	case codes.PermissionDenied:
		message = "Control API denied permission for this actor"
	case codes.Unauthenticated:
		message = "Control API authentication failed"
	case codes.NotFound:
		message = "actor was not found in the configured atespace"
	case codes.DeadlineExceeded:
		message = "lifecycle operation timed out"
	case codes.Canceled:
		message = "lifecycle operation was canceled"
	case codes.Unavailable:
		message = "Control API is unavailable"
	default:
		message = "lifecycle operation failed (" + status.Code(err).String() + ")"
	}
	if len(message) > maxOutcomeMessage {
		return message[:maxOutcomeMessage]
	}
	return message
}

func randomID() string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("op-%d", time.Now().UnixNano())
	}
	return "op-" + hex.EncodeToString(buffer)
}
