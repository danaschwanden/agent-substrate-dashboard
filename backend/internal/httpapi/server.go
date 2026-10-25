package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"
)

const maxRequestBodyBytes = 64 << 10

// SnapshotProvider is the complete backend state needed by the HTTP boundary.
type SnapshotProvider interface {
	Snapshot() DashboardSnapshot
	Ready() bool
}

// OperationExecutor performs a non-atomic lifecycle operation for explicit targets.
type OperationExecutor interface {
	Execute(context.Context, OperationType, []string) DashboardOperation
}

// Server implements the generated OpenAPI router interface.
type Server struct {
	provider SnapshotProvider
	executor OperationExecutor
}

var _ ServerInterface = (*Server)(nil)

func NewServer(provider SnapshotProvider, executor OperationExecutor) *Server {
	return &Server{provider: provider, executor: executor}
}

func (s *Server) GetMeta(w http.ResponseWriter, _ *http.Request) {
	snapshot := s.provider.Snapshot()
	writeJSON(w, http.StatusOK, Meta{
		ApiVersion:                snapshot.ApiVersion,
		ComponentVersion:          snapshot.ComponentVersion,
		SupportedSubstrateVersion: snapshot.SupportedSubstrateVersion,
		Capabilities:              append([]Capability(nil), snapshot.Capabilities...),
		Scope:                     snapshot.Scope,
	})
}

func (s *Server) GetSnapshot(w http.ResponseWriter, _ *http.Request) {
	if !s.provider.Ready() {
		snapshot := s.provider.Snapshot()
		message := snapshot.Connection.Message
		if message == "" {
			message = "No successful scoped Control API observation is available yet"
		}
		writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{Code: "OBSERVATION_UNAVAILABLE", Message: message})
		return
	}
	writeJSON(w, http.StatusOK, s.provider.Snapshot())
}

func (*Server) GetLiveness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, Health{Status: Ok})
}

func (s *Server) GetReadiness(w http.ResponseWriter, _ *http.Request) {
	if !s.provider.Ready() {
		writeJSON(w, http.StatusServiceUnavailable, Health{Status: NotReady})
		return
	}
	writeJSON(w, http.StatusOK, Health{Status: Ok})
}

func (s *Server) ResumeActors(w http.ResponseWriter, r *http.Request) {
	s.executeOperation(w, r, OperationTypeRESUME)
}

func (s *Server) SuspendActors(w http.ResponseWriter, r *http.Request) {
	s.executeOperation(w, r, OperationTypeSUSPEND)
}

func (s *Server) executeOperation(w http.ResponseWriter, r *http.Request, operationType OperationType) {
	actorNames, valid := operationTargets(r)
	if !valid {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Code: "INVALID_REQUEST", Message: "Request does not match the API contract"})
		return
	}
	if s.executor == nil {
		writeJSON(w, http.StatusForbidden, ErrorResponse{
			Code: "OBSERVE_ONLY", Message: "Lifecycle operations are disabled; this backend advertises only the OBSERVE capability",
		})
		return
	}
	writeJSON(w, http.StatusOK, BulkOperationResponse{Operation: s.executor.Execute(r.Context(), operationType, actorNames)})
}

func operationTargets(r *http.Request) ([]string, bool) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return nil, false
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request BulkOperationRequest
	if err := decoder.Decode(&request); err != nil || len(request.ActorNames) == 0 || len(request.ActorNames) > 500 {
		return nil, false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, false
	}
	seen := make(map[string]struct{}, len(request.ActorNames))
	for _, name := range request.ActorNames {
		if name == "" || strings.TrimSpace(name) != name {
			return nil, false
		}
		if _, duplicate := seen[name]; duplicate {
			return nil, false
		}
		seen[name] = struct{}{}
	}
	return append([]string(nil), request.ActorNames...), true
}

// NewHandler returns the generated router with metadata-only structured
// request logs. It never logs request or response bodies.
func NewHandler(server *Server, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return loggingMiddleware(logger, HandlerWithOptions(server, StdHTTPServerOptions{
		ErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, _ error) {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Code: "INVALID_REQUEST", Message: "Request does not match the API contract"})
		},
	}))
}

func writeJSON(w http.ResponseWriter, status int, response any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response)
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(status int) {
	if r.status != 0 {
		return
	}
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(body []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(body)
}

func loggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		w.Header().Set("Cache-Control", "no-store")
		recorder := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		logger.InfoContext(r.Context(), "HTTP request completed",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", status),
			slog.Duration("duration", time.Since(started)),
		)
	})
}
