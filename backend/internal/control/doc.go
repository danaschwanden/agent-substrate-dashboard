// Package control adapts the Agent Substrate gRPC Control API.
package control

import ateapi "go.ate.dev/substrate/api/ateapi/v1"

// SupportedSubstrateVersion is the exact module version validated by this release.
const SupportedSubstrateVersion = "v0.6.1-0.20261023032142-4ebd668d62e3"

// Client is the typed Agent Substrate Control API boundary used by the adapter.
type Client = ateapi.ControlClient
