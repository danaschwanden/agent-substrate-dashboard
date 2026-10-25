// Package collector projects the Agent Substrate Control API into dashboard data.
package collector

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/danaschwanden/agent-substrate-dashboard/backend/internal/dashboard"
	"github.com/danaschwanden/agent-substrate-dashboard/backend/internal/httpapi"
	ateapi "go.ate.dev/substrate/api/ateapi/v1"
	"google.golang.org/grpc"
)

const defaultPageSize int32 = 500

// ControlClient is the read-only Control API surface needed by the dashboard.
type ControlClient interface {
	ListActors(context.Context, *ateapi.ListActorsRequest, ...grpc.CallOption) (*ateapi.ListActorsResponse, error)
	ListWorkers(context.Context, *ateapi.ListWorkersRequest, ...grpc.CallOption) (*ateapi.ListWorkersResponse, error)
	ListWorkerActorAssignments(context.Context, *ateapi.ListWorkerActorAssignmentsRequest, ...grpc.CallOption) (*ateapi.ListWorkerActorAssignmentsResponse, error)
}

// Config fixes one dashboard instance to one atespace and worker pool.
type Config struct {
	Atespace   string
	WorkerPool string
	PageSize   int32
	Now        func() time.Time
}

// Collector performs complete paginated reads and produces an allowlisted projection.
type Collector struct {
	client ControlClient
	config Config
}

func New(client ControlClient, config Config) (*Collector, error) {
	if client == nil {
		return nil, fmt.Errorf("Control API client is required")
	}
	if config.Atespace == "" {
		return nil, fmt.Errorf("atespace is required")
	}
	if config.WorkerPool == "" {
		return nil, fmt.Errorf("worker pool is required")
	}
	if config.PageSize <= 0 || config.PageSize > 1000 {
		config.PageSize = defaultPageSize
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	return &Collector{client: client, config: config}, nil
}

func (c *Collector) Observe(ctx context.Context) (dashboard.Observation, error) {
	actors, err := c.listActors(ctx)
	if err != nil {
		return dashboard.Observation{}, fmt.Errorf("list actors: %w", err)
	}
	workers, err := c.listWorkers(ctx)
	if err != nil {
		return dashboard.Observation{}, fmt.Errorf("list workers: %w", err)
	}

	projectedWorkers := make([]httpapi.Worker, 0, len(workers))
	for _, worker := range workers {
		assignments, err := c.listAssignments(ctx, worker.GetMetadata().GetName())
		if err != nil {
			return dashboard.Observation{}, fmt.Errorf("list assignments for worker %q: %w", worker.GetMetadata().GetName(), err)
		}
		projectedWorkers = append(projectedWorkers, projectWorker(worker, assignments, c.config.Atespace))
	}
	projectedActors := make([]httpapi.Actor, 0, len(actors))
	for _, actor := range actors {
		projectedActors = append(projectedActors, projectActor(actor))
	}
	sort.Slice(projectedActors, func(i, j int) bool { return projectedActors[i].Name < projectedActors[j].Name })
	sort.Slice(projectedWorkers, func(i, j int) bool { return projectedWorkers[i].Name < projectedWorkers[j].Name })
	return dashboard.Observation{ObservedAt: c.config.Now().UTC(), Actors: projectedActors, Workers: projectedWorkers}, nil
}

func (c *Collector) listActors(ctx context.Context) ([]*ateapi.Actor, error) {
	var all []*ateapi.Actor
	err := pages(func(token string) (string, error) {
		response, err := c.client.ListActors(ctx, &ateapi.ListActorsRequest{
			Atespace: c.config.Atespace,
			Options:  &ateapi.ListOptions{PageSize: c.config.PageSize, PageToken: token},
		})
		if err != nil {
			return "", err
		}
		for _, actor := range response.GetActors() {
			if actor.GetMetadata().GetAtespace() == c.config.Atespace {
				all = append(all, actor)
			}
		}
		return response.GetNextPageToken(), nil
	})
	return all, err
}

func (c *Collector) listWorkers(ctx context.Context) ([]*ateapi.Worker, error) {
	var all []*ateapi.Worker
	err := pages(func(token string) (string, error) {
		response, err := c.client.ListWorkers(ctx, &ateapi.ListWorkersRequest{
			Options: &ateapi.ListOptions{PageSize: c.config.PageSize, PageToken: token},
		})
		if err != nil {
			return "", err
		}
		for _, worker := range response.GetWorkers() {
			if worker.GetWorkerPool() == c.config.WorkerPool {
				all = append(all, worker)
			}
		}
		return response.GetNextPageToken(), nil
	})
	return all, err
}

func (c *Collector) listAssignments(ctx context.Context, workerName string) ([]*ateapi.ActorAssignment, error) {
	var all []*ateapi.ActorAssignment
	err := pages(func(token string) (string, error) {
		response, err := c.client.ListWorkerActorAssignments(ctx, &ateapi.ListWorkerActorAssignmentsRequest{
			Worker:  &ateapi.ObjectRef{Name: workerName},
			Options: &ateapi.ListOptions{PageSize: c.config.PageSize, PageToken: token},
		})
		if err != nil {
			return "", err
		}
		for _, assignment := range response.GetActorAssignments() {
			if assignment.GetActor().GetAtespace() == c.config.Atespace {
				all = append(all, assignment)
			}
		}
		return response.GetNextPageToken(), nil
	})
	return all, err
}

func pages(next func(string) (string, error)) error {
	token := ""
	seen := map[string]struct{}{}
	for {
		nextToken, err := next(token)
		if err != nil {
			return err
		}
		if nextToken == "" {
			return nil
		}
		if _, exists := seen[nextToken]; exists || nextToken == token {
			return fmt.Errorf("Control API repeated pagination token %q", nextToken)
		}
		seen[nextToken] = struct{}{}
		token = nextToken
	}
}

func projectActor(actor *ateapi.Actor) httpapi.Actor {
	result := httpapi.Actor{
		Name:     actor.GetMetadata().GetName(),
		Atespace: actor.GetMetadata().GetAtespace(),
		State:    normalizeActorState(actor.GetStatus().GetState()),
	}
	if name := actor.GetStatus().GetWorkerAssignment().GetWorker().GetName(); name != "" {
		result.WorkerName = &name
	}
	if node := actor.GetStatus().GetAssignedNode(); node != "" {
		result.AssignedNode = &node
	}
	if result.State == httpapi.ActorStateCRASHED {
		if summary := bounded(actor.GetStatus().GetCrash().GetMessage(), 256); summary != "" {
			result.CrashSummary = &summary
		}
	}
	return result
}

func projectWorker(worker *ateapi.Worker, assignments []*ateapi.ActorAssignment, atespace string) httpapi.Worker {
	actorNames := make([]string, 0, len(assignments))
	seen := map[string]struct{}{}
	for _, assignment := range assignments {
		actor := assignment.GetActor()
		if actor.GetAtespace() != atespace || actor.GetName() == "" {
			continue
		}
		if _, exists := seen[actor.GetName()]; exists {
			continue
		}
		seen[actor.GetName()] = struct{}{}
		actorNames = append(actorNames, actor.GetName())
	}
	sort.Strings(actorNames)
	status := worker.GetStatus()
	capacity, allocated := status.GetCapacity(), status.GetAllocated()
	return httpapi.Worker{
		Name:       worker.GetMetadata().GetName(),
		Namespace:  worker.GetWorkerNamespace(),
		Pool:       worker.GetWorkerPool(),
		Pod:        worker.GetWorkerPod(),
		Node:       worker.GetNodeName(),
		State:      normalizeWorkerState(status.GetState()),
		ActorNames: actorNames,
		Resources: []httpapi.ResourceAllocation{
			{Name: "cpu", Unit: "millicores", Capacity: strconv.FormatInt(capacity.GetCpuMillicores(), 10), Allocated: strconv.FormatInt(allocated.GetCpuMillicores(), 10)},
			{Name: "memory", Unit: "bytes", Capacity: strconv.FormatInt(capacity.GetMemoryBytes(), 10), Allocated: strconv.FormatInt(allocated.GetMemoryBytes(), 10)},
			{Name: "actors", Unit: "count", Capacity: strconv.FormatInt(int64(capacity.GetActors()), 10), Allocated: strconv.FormatInt(int64(allocated.GetActors()), 10)},
		},
	}
}

func normalizeActorState(state ateapi.ActorState) httpapi.ActorState {
	switch state {
	case ateapi.ActorState_ACTOR_STATE_RESUMING:
		return httpapi.ActorStateRESUMING
	case ateapi.ActorState_ACTOR_STATE_RUNNING:
		return httpapi.ActorStateRUNNING
	case ateapi.ActorState_ACTOR_STATE_SUSPENDING:
		return httpapi.ActorStateSUSPENDING
	case ateapi.ActorState_ACTOR_STATE_SUSPENDED:
		return httpapi.ActorStateSUSPENDED
	case ateapi.ActorState_ACTOR_STATE_CRASHED:
		return httpapi.ActorStateCRASHED
	case ateapi.ActorState_ACTOR_STATE_DELETING:
		return httpapi.ActorStateDELETING
	case ateapi.ActorState_ACTOR_STATE_REVERTING:
		return httpapi.ActorStateREVERTING
	default:
		return httpapi.ActorStateUNKNOWN
	}
}

func normalizeWorkerState(state ateapi.WorkerState) httpapi.WorkerState {
	switch state {
	case ateapi.WorkerState_WORKER_STATE_ACTIVE:
		return httpapi.WorkerStateACTIVE
	case ateapi.WorkerState_WORKER_STATE_DRAINING:
		return httpapi.WorkerStateDRAINING
	case ateapi.WorkerState_WORKER_STATE_UNAVAILABLE:
		return httpapi.WorkerStateUNAVAILABLE
	default:
		return httpapi.WorkerStateUNKNOWN
	}
}

func bounded(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
