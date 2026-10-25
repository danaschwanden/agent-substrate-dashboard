// Copyright 2026 Agent Substrate Dashboard Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/danaschwanden/agent-substrate-dashboard/backend/internal/collector"
	"github.com/danaschwanden/agent-substrate-dashboard/backend/internal/config"
	"github.com/danaschwanden/agent-substrate-dashboard/backend/internal/control"
	"github.com/danaschwanden/agent-substrate-dashboard/backend/internal/dashboard"
	"github.com/danaschwanden/agent-substrate-dashboard/backend/internal/httpapi"
	"github.com/danaschwanden/agent-substrate-dashboard/backend/internal/lifecycle"
	"google.golang.org/grpc/status"
)

var version = "development"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("Dashboard backend stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	runtimeConfig, err := config.Load(version)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	connection, err := control.Dial(ctx, runtimeConfig.Control)
	if err != nil {
		return err
	}
	defer connection.Close()

	observer, err := collector.New(connection, collector.Config{
		Atespace: runtimeConfig.Scope.Atespace, WorkerPool: runtimeConfig.Scope.WorkerPool, PageSize: runtimeConfig.PageSize,
	})
	if err != nil {
		return err
	}
	capabilities := []httpapi.Capability{httpapi.CapabilityOBSERVE}
	if runtimeConfig.EnableOperations {
		capabilities = append(capabilities, httpapi.CapabilityRESUME, httpapi.CapabilitySUSPEND)
	}
	core := dashboard.New(observer, dashboard.Config{
		Scope:              runtimeConfig.Scope,
		ComponentVersion:   runtimeConfig.ComponentVersion,
		ActorLimit:         runtimeConfig.ActorLimit,
		WorkerLimit:        runtimeConfig.WorkerLimit,
		TransitionLimit:    runtimeConfig.TransitionLimit,
		OperationLimit:     runtimeConfig.OperationLimit,
		Capabilities:       capabilities,
		DensityWindow:      runtimeConfig.DensityWindow,
		PollInterval:       runtimeConfig.PollInterval,
		ObservationTimeout: runtimeConfig.ObservationTimeout,
		OnObservationError: func(err error) {
			logger.Warn("Control API observation failed", slog.String("control_code", status.Code(err).String()))
		},
	}, nil)
	var operationExecutor httpapi.OperationExecutor
	if runtimeConfig.EnableOperations {
		operationExecutor, err = lifecycle.New(connection, core, lifecycle.Config{
			Atespace: runtimeConfig.Scope.Atespace, Concurrency: runtimeConfig.OperationConcurrency, Timeout: runtimeConfig.OperationTimeout,
		})
		if err != nil {
			return err
		}
	}

	pollingDone := make(chan error, 1)
	go func() { pollingDone <- core.Run(ctx) }()

	httpServer := &http.Server{
		Addr:              runtimeConfig.ListenAddress,
		Handler:           httpapi.NewHandler(httpapi.NewServer(core, operationExecutor), logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	httpDone := make(chan error, 1)
	go func() {
		logger.Info("Dashboard backend listening",
			slog.String("address", runtimeConfig.ListenAddress),
			slog.String("atespace", runtimeConfig.Scope.Atespace),
			slog.String("worker_pool", runtimeConfig.Scope.WorkerPool),
			slog.String("component_version", runtimeConfig.ComponentVersion),
		)
		httpDone <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-httpDone:
		stop()
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case err := <-pollingDone:
		if !errors.Is(err, context.Canceled) {
			return err
		}
	case <-ctx.Done():
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownContext); err != nil {
		return err
	}
	logger.Info("Dashboard backend shut down")
	return nil
}
