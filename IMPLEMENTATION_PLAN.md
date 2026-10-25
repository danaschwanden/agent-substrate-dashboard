# Implementation plan

Each phase ends with a usable, reviewable seam. Do not start the next phase until its exit criteria pass.

## Phase 1: Establish the repository and contracts

1. Create `agent-substrate-dashboard` with Apache-2.0, Go, Node, TypeScript, Vite, and Lit configuration.
2. Add the proposed directory boundaries and copy the glossary and accepted ADRs.
3. Pin the supported Agent Substrate module version.
4. Write `api/openapi.yaml` for metadata, snapshots, capabilities, truncation, and bulk operation results.
5. Add deterministic generation for Go transport types and the TypeScript client.
6. Define canonical fixtures for idle, active, mixed, crashed, saturated, stale, truncated, and partial-failure fleets.

Exit criteria:

- both generated clients compile;
- regeneration is reproducible and leaves no diff;
- the schema documents every field and enum;
- fixtures validate against OpenAPI.

## Phase 2: Build the backend observation core

1. Add the direct gRPC Control client with projected-token and trust-bundle authentication.
2. Implement complete pagination for actors, workers, and assignments.
3. Apply the atespace and worker-pool scope.
4. Normalize lifecycle and worker states, including Unknown values.
5. Join zero-to-many actor assignments to workers.
6. Implement the polling loop with non-overlapping requests and cancellation.
7. Infer and retain bounded Observed Transitions.
8. Calculate Fleet Ratio, Observed Density, sample age, and resource summaries.
9. Enforce display limits and emit truncation metadata.

Exit criteria:

- an in-process fake Control service covers every state and pagination path;
- stale Control API data is reported without inventing health claims;
- metric tests cover zero workers, zero occupied workers, changing actor counts, and incomplete windows;
- no subprocess or Kubernetes API dependency exists in the backend.

## Phase 3: Expose the versioned backend API

1. Implement `/api/v1/meta`, `/api/v1/snapshot`, `/healthz`, and `/readyz` from generated transport types.
2. Advertise observe-only capabilities by default.
3. Add bounded in-memory retention for transitions and operations.
4. Reject unsupported configuration and missing permissions clearly.
5. Return only allowlisted fields and bounded failure text.
6. Add structured logs without actor payloads or credentials.

Exit criteria:

- OpenAPI conformance tests pass;
- readiness remains false until the first successful scoped observation;
- API and component versions are present in every diagnostic response;
- raw protobuf resources and labels cannot leak through serialization.

## Phase 4: Build the Lit component system against fixtures

1. Implement design tokens, the responsive shell, and the Reference Viewport.
2. Implement `<substrate-dashboard>` with compatibility checks, abortable polling, stale-state retention, and selection state.
3. Implement each semantic card as its own Shadow-DOM Lit element.
4. Keep cards network-agnostic and communicate through typed properties and custom events.
5. Add the fixture gallery with independent examples for every card.
6. Add keyboard selection, visible focus, semantic labels, text state indicators, and reduced-motion behavior.

Exit criteria:

- every card renders independently from fixtures;
- no card imports the API client;
- the full 1280×1080 baseline contains all sections without page scrolling;
- container-width tests prove cards do not depend on a fixed page width;
- Playwright baselines cover the agreed state matrix.

## Phase 5: Add lifecycle operations

1. Add explicit actor multi-selection in `<actor-fleet-card>`.
2. Emit semantic Resume and Suspend requests to the shell.
3. Implement backend operations with bounded parallel gRPC calls.
4. Return and display one result per actor, including partial failures.
5. Retain Dashboard Operations in memory and show them in the operation stream.
6. Keep controls visible but disabled with a reason in observe-only mode.

Exit criteria:

- no operation can target actors outside the configured atespace;
- empty and implicit fleet-wide targets are rejected;
- partial success is visible and never reported as an atomic success;
- fake-gRPC tests cover concurrent state changes and permission denial.

## Phase 6: Package the two deployments

1. Build minimal multi-stage frontend and backend images.
2. Configure the frontend server to proxy `/api/v1/*` to the backend service.
3. Add health probes, resource defaults, non-root security contexts, and read-only filesystems where practical.
4. Create the Kustomize base and an example kind overlay.
5. Document viewer and editor AccessPolicy requirements without automating policy escalation.
6. Build both `linux/amd64` and `linux/arm64` locally or in CI.

Exit criteria:

- only the frontend Service needs browser exposure;
- observe-only installation works with viewer permissions;
- operation mode works only with explicit enablement and editor permissions;
- image digests can be pinned entirely from a consumer overlay.

## Phase 7: Prove the first integration

1. Add a dashboard overlay to `herdr-agent-substrate-on-kind` without importing Herdr concepts.
2. Grant a dedicated dashboard ServiceAccount access to the `herdr-agents` atespace.
3. Verify actors, workers, multiple assignments, transitions, and density against the real Control API.
4. Verify Resume and Suspend through the dashboard.
5. Exercise frontend/backend version mismatch and Control API failure states.
6. Record the supported Substrate commit in release metadata.

Exit criteria:

- the kind walkthrough can install and remove the dashboard independently;
- the dashboard observes Herdr-driven lifecycle changes without calling Herdr;
- the acceptance suite passes on amd64 and arm64 where CI capacity permits;
- all UI claims can be reconciled with Control API responses.

## Phase 8: Release v0.1.0

1. Publish matching frontend and backend images from one tag.
2. Publish the matching Kustomize base and compatibility statement.
3. Run contract, backend, visual, image, and kind acceptance suites.
4. Document the trusted-network constraint, observe-only default, scale limits, and ephemeral history.

The release is complete when a new consumer can pin the artifacts, supply a scoped overlay and AccessPolicy, and reach a truthful dashboard without modifying either component.
