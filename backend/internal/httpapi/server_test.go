package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
)

type fakeSnapshotProvider struct {
	ready    bool
	snapshot DashboardSnapshot
}

type fakeOperationExecutor struct {
	operationType OperationType
	actorNames    []string
	operation     DashboardOperation
}

func (e *fakeOperationExecutor) Execute(_ context.Context, operationType OperationType, actorNames []string) DashboardOperation {
	e.operationType = operationType
	e.actorNames = append([]string(nil), actorNames...)
	return e.operation
}

func (p *fakeSnapshotProvider) Snapshot() DashboardSnapshot { return p.snapshot }
func (p *fakeSnapshotProvider) Ready() bool                 { return p.ready }

func fixtureSnapshot() DashboardSnapshot {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	value := 2.0
	return DashboardSnapshot{
		ApiVersion:                V1,
		ComponentVersion:          "v0.1.0",
		SupportedSubstrateVersion: "pinned",
		BackendStartedAt:          now,
		ObservedAt:                now,
		Scope:                     Scope{Atespace: "agents", WorkerPool: "default"},
		Capabilities:              []Capability{CapabilityOBSERVE},
		Connection:                ControlApiConnection{State: ConnectionStateCONNECTED, Message: "", LastSuccessfulObservationAt: &now},
		ActorSummary:              ActorSummary{Total: 1, States: []ActorStateCount{{State: ActorStateRUNNING, Count: 1}}},
		WorkerSummary:             WorkerSummary{Total: 1, Occupied: 1, Assignments: 1, States: []WorkerStateCount{{State: WorkerStateACTIVE, Count: 1}}},
		TransitionSummary:         TransitionSummary{},
		Efficiency:                Efficiency{FleetRatio: Ratio{Numerator: 1, Denominator: 1, Value: &value}, ObservedDensity: ObservedDensity{Available: true, Value: &value, ActorsAtPeak: 1, OccupiedWorkersAtPeak: 1, WindowSeconds: 900}},
		Truncation:                Truncation{Actors: TruncationDetail{Total: 1, Returned: 1, Limit: 500}, Workers: TruncationDetail{Total: 1, Returned: 1, Limit: 100}},
		Actors:                    []Actor{{Name: "actor-a", Atespace: "agents", State: ActorStateRUNNING}},
		Workers:                   []Worker{{Name: "opaque-worker", Namespace: "workers", Pool: "default", Pod: "worker-pod", Node: "node-a", State: WorkerStateACTIVE, ActorNames: []string{"actor-a"}, Resources: []ResourceAllocation{}}},
		Transitions:               []ObservedTransition{},
		Operations:                []DashboardOperation{},
	}
}

func TestEndpointsReturnContractResponses(t *testing.T) {
	provider := &fakeSnapshotProvider{snapshot: fixtureSnapshot()}
	handler := NewHandler(NewServer(provider, nil), slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		status     int
		schemaName string
	}{
		{name: "metadata before ready", method: http.MethodGet, path: "/api/v1/meta", status: http.StatusOK, schemaName: "Meta"},
		{name: "snapshot unavailable", method: http.MethodGet, path: "/api/v1/snapshot", status: http.StatusServiceUnavailable, schemaName: "ErrorResponse"},
		{name: "liveness", method: http.MethodGet, path: "/healthz", status: http.StatusOK, schemaName: "Health"},
		{name: "readiness before observation", method: http.MethodGet, path: "/readyz", status: http.StatusServiceUnavailable, schemaName: "Health"},
		{name: "resume disabled", method: http.MethodPost, path: "/api/v1/operations/resume", body: `{"actorNames":["actor-a"]}`, status: http.StatusForbidden, schemaName: "ErrorResponse"},
		{name: "suspend disabled", method: http.MethodPost, path: "/api/v1/operations/suspend", body: `{"actorNames":["actor-a"]}`, status: http.StatusForbidden, schemaName: "ErrorResponse"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			if test.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.status, response.Body.String())
			}
			validateSchema(t, test.schemaName, response.Body.Bytes())
		})
	}

	provider.ready = true
	for _, endpoint := range []struct {
		path, schema string
	}{
		{path: "/api/v1/snapshot", schema: "DashboardSnapshot"},
		{path: "/readyz", schema: "Health"},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, endpoint.path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d; body=%s", endpoint.path, response.Code, response.Body.String())
		}
		validateSchema(t, endpoint.schema, response.Body.Bytes())
	}
}

func TestMalformedRequestIsBoundedAndLogsNoPayload(t *testing.T) {
	provider := &fakeSnapshotProvider{snapshot: fixtureSnapshot()}
	var logs bytes.Buffer
	handler := NewHandler(NewServer(provider, nil), slog.New(slog.NewJSONHandler(&logs, nil)))
	secret := "actor-name-that-must-not-be-logged"
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/operations/resume", strings.NewReader(`{"actorNames":["`+secret+`"]`))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	if response.Body.Len() > 256 {
		t.Fatalf("error response has %d bytes, want bounded response", response.Body.Len())
	}
	if strings.Contains(logs.String(), secret) || strings.Contains(response.Body.String(), secret) {
		t.Fatalf("request payload leaked: response=%s logs=%s", response.Body.String(), logs.String())
	}
	validateSchema(t, "ErrorResponse", response.Body.Bytes())
}

func TestEnabledOperationsReturnPerActorResults(t *testing.T) {
	provider := &fakeSnapshotProvider{ready: true, snapshot: fixtureSnapshot()}
	now := time.Date(2026, 10, 25, 12, 1, 0, 0, time.UTC)
	executor := &fakeOperationExecutor{operation: DashboardOperation{
		Id: "op-1", Type: OperationTypeRESUME, RequestedAt: now, CompletedAt: now,
		Results: []ActorOperationResult{
			{ActorName: "actor-a", Outcome: SUCCEEDED, Message: ""},
			{ActorName: "actor-b", Outcome: FAILED, Message: "actor state changed concurrently"},
		},
	}}
	handler := NewHandler(NewServer(provider, executor), slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/operations/resume", strings.NewReader(`{"actorNames":["actor-a","actor-b"]}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	if executor.operationType != OperationTypeRESUME || strings.Join(executor.actorNames, ",") != "actor-a,actor-b" {
		t.Fatalf("executor call = %s %v", executor.operationType, executor.actorNames)
	}
	validateSchema(t, "BulkOperationResponse", response.Body.Bytes())
}

func TestOperationTargetsMustBeExplicitUniqueAndBounded(t *testing.T) {
	provider := &fakeSnapshotProvider{snapshot: fixtureSnapshot()}
	handler := NewHandler(NewServer(provider, &fakeOperationExecutor{}), slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	for _, body := range []string{
		`{"actorNames":[]}`,
		`{"actorNames":["actor-a","actor-a"]}`,
		`{"actorNames":[""]}`,
		`{"actorNames":[" actor-a"]}`,
		`{"actorNames":["actor-a"],"all":true}`,
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/operations/suspend", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want 400", body, response.Code)
		}
		validateSchema(t, "ErrorResponse", response.Body.Bytes())
	}
}

func validateSchema(t *testing.T, schemaName string, body []byte) {
	t.Helper()
	loader := openapi3.NewLoader()
	document, err := loader.LoadFromFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := document.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, body)
	}
	schema := document.Components.Schemas[schemaName]
	if schema == nil || schema.Value == nil {
		t.Fatalf("schema %q not found", schemaName)
	}
	if err := schema.Value.VisitJSON(value); err != nil {
		t.Fatalf("response violates %s: %v; body=%s", schemaName, err, body)
	}
}
