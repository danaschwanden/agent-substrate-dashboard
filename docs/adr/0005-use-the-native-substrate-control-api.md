# Use the native Agent Substrate Control API

The Dashboard Backend will call Agent Substrate’s typed gRPC `Control` API for actors, workers, assignments, and lifecycle operations. It will not execute or parse `kubectl-ate`, which would couple the standalone dashboard to a CLI binary, its output format, and shell execution behavior.
