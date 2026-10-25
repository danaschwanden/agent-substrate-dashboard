# Agent Substrate Dashboard

Language for the operational dashboard that visualizes the relationship between logical actors and the physical worker pool.

## Language

**Dashboard Snapshot**:
A coherent observation of the Control API connection, actors, workers, Observed Transitions, Dashboard Operations, and derived fleet statistics returned by one state request.
_Avoid_: Component state, panel data

**Stale Snapshot**:
The last successful Dashboard Snapshot displayed after a newer state request fails, visibly marked with the time since its observation.
_Avoid_: Cached data, offline state

**Reference Viewport**:
The 1280×1080 presentation size used to verify screenshot parity, not a fixed size imposed on the dashboard or its sections.
_Avoid_: Canvas size, fixed viewport

**Dashboard Section**:
An independently replaceable visual region that receives data from the dashboard shell and represents one operational concern.
_Avoid_: Widget, page fragment

**Trusted Dashboard**:
The operational surface for observing the fleet and issuing permitted lifecycle operations, available only within an explicitly restricted network boundary.
_Avoid_: Read-only dashboard, public dashboard

**Dashboard Frontend**:
The portable Lit application that renders Dashboard Snapshots and emits dashboard commands without communicating with Agent Substrate directly.
_Avoid_: OpenClaw dashboard, demo page

**Dashboard Backend**:
The portable service that translates Agent Substrate state and operations into the dashboard’s stable HTTP contract.
_Avoid_: OpenClaw backend, UI server

**Substrate Fleet**:
The Agent Substrate actors, selected worker capacity, and lifecycle transitions within the Dashboard Scope.
_Avoid_: Herdr fleet, agent client state

**Dashboard Scope**:
One configured atespace and one selected worker pool observed by a Dashboard Backend instance.
_Avoid_: Cluster, deployment, all atespaces

**Observed Transition**:
An actor lifecycle state change detected between two backend observations and retained only since the backend started.
_Avoid_: Audit event, control-plane event

**Dashboard Operation**:
A lifecycle command submitted through the Dashboard Backend and retained only since the backend started.
_Avoid_: Substrate event, audit record

**Bulk Lifecycle Operation**:
A set of independent Resume or Suspend requests for explicitly selected actors, reported with one outcome per actor and no transactional guarantee.
_Avoid_: Fleet transaction, batch state change

**Worker Occupancy**:
The set of actors assigned to a worker together with its allocated and total resources; a worker may host zero, one, or multiple actors.
_Avoid_: Occupying actor, one actor per worker

**Fleet Ratio**:
The current number of managed actors divided by the number of selected workers in the Dashboard Scope.
_Avoid_: Cost reduction, savings ratio

**Observed Density**:
The number of managed actors at the moment of peak occupied-worker demand divided by the occupied workers at that moment, measured over a stated rolling window.
_Avoid_: Average utilization, configured density

**Control API Connection**:
The Dashboard Backend’s current ability to authenticate to and query the Agent Substrate Control API.
_Avoid_: Control-plane health, cluster health
