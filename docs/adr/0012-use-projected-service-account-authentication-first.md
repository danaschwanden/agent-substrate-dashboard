# Support projected ServiceAccount authentication first

The first Dashboard Backend release will connect to Agent Substrate from inside Kubernetes using a projected ServiceAccount token and the mounted Substrate trust bundle. This matches the first kind-based integration and supports scoped atespace authorization without kubeconfigs or token-minting subprocesses; other gRPC authentication mechanisms can be added through the backend client boundary later.
