# Research: installing the dashboard on a kind-based Agent Substrate cluster

Date: 2026-10-25

## Scope and source limitation

The referenced Herdr integration repository was not available to unauthenticated web or API requests, and no checkout was available in the workspace. Consequently, exact repository-specific script names and Herdr atespace/WorkerPool names could not be verified from that source. The installation guide should avoid inventing them: it should make the cluster context, atespace, and WorkerPool explicit inputs and show commands that discover the latter two.

The findings below use the current dashboard repository plus current first-party Agent Substrate, kind, and Kagent integration sources.

## Confirmed findings

### 1. Cluster and control-plane expectations

- Agent Substrate's official kind creator defaults `KIND_CLUSTER_NAME` to `kind`, which yields kube context `kind-kind`. It supports overriding `KIND_CLUSTER_NAME`, so dashboard instructions must not hard-code the default. The same script creates the feature-gated kind cluster and local registry. [Agent Substrate `create-kind-cluster.sh`](https://github.com/agent-substrate/substrate/blob/main/hack/create-kind-cluster.sh)
- The official development quickstart creates the cluster with `hack/create-kind-cluster.sh`, installs the system with `hack/install-ate-kind.sh deploy ate-system`, and installs `kubectl-ate` from source. [Agent Substrate README](https://github.com/agent-substrate/substrate#quickstart-development)
- The kind configuration explicitly enables `ClusterTrustBundle`, `ClusterTrustBundleProjection`, and `PodCertificateRequest`, and serves `certificates.k8s.io/v1beta1`. A dashboard Pod that projects the ServiceDNS ClusterTrustBundle therefore depends on the same correctly configured cluster. [Agent Substrate `create-kind-cluster.sh`](https://github.com/agent-substrate/substrate/blob/main/hack/create-kind-cluster.sh)
- The in-cluster control endpoint is `api.ate-system.svc:443`. The dashboard base already uses it as both endpoint and TLS server name and projects a token with audience `api.ate-system.svc`.
- Substrate v0.6 restricts API ingress to namespaces labeled `ate.dev/api-access=true`. The dashboard base now includes that label; without it, the backend can authenticate correctly but still time out at the NetworkPolicy boundary. [Kagent's Agent Substrate installation notes](https://github.com/kagent-dev/kagent/blob/main/helm/README.md)

### 2. Dashboard target selection must be explicit

- A dashboard instance is intentionally fixed to exactly one atespace and one WorkerPool (`DASHBOARD_ATESPACE` and `DASHBOARD_WORKER_POOL`). The existing kind overlay hard-codes `demo` and `agents`; these are examples, not safe defaults for an arbitrary Herdr cluster.
- Operators can discover atespaces with `kubectl ate get atespaces` and pools with `kubectl get workerpools.ate.dev -A`. The official counter demo, for example, uses atespace `ate-demo-counter` and a pool named `counter`, so the current example values would render an empty dashboard. [Agent Substrate README](https://github.com/agent-substrate/substrate#quickstart-development) and [`kubectl-ate` command reference](https://github.com/agent-substrate/substrate/blob/main/cmd/kubectl-ate/README.md)
- Actors and workers are Control API records, not Kubernetes CRDs; verification must use `kubectl ate get actors -a "$ATESPACE"` and `kubectl ate get workers`. [`kubectl-ate` command reference](https://github.com/agent-substrate/substrate/blob/main/cmd/kubectl-ate/README.md)

### 3. Local images and browser access

- For a local checkout, the simplest deterministic loop is to build `agent-substrate-dashboard-backend:dev` and `agent-substrate-dashboard-frontend:dev`, then run `kind load docker-image ... --name "$KIND_CLUSTER_NAME"`. This matches the existing kind overlay and its `IfNotPresent` pull policy. kind officially supports loading one or more local Docker images into a named cluster. [kind quick start](https://kind.sigs.k8s.io/docs/user/quick-start/#loading-an-image-into-your-cluster)
- kind v0.26.0 cannot detect the containerd v2 snapshotter configuration used by newer node images. The same `failed to detect containerd snapshotter` failure is tracked upstream and fixed in the v0.27.0 milestone, so the installer must reject older kind clients before building images. [kind issue #3853](https://github.com/kubernetes-sigs/kind/issues/3853)
- A fixed NodePort is not portable unless the kind configuration maps that port to the host. The dashboard overlay now keeps both Services internal and uses `kubectl port-forward -n agent-substrate-dashboard svc/agent-substrate-dashboard-frontend 8090:80` as the reliable cross-platform access path. The upstream Substrate quickstart likewise uses `kubectl port-forward` for host access. [Agent Substrate README](https://github.com/agent-substrate/substrate#quickstart-development)

### 4. Authorization requirements and safe policy workflow

- A Kubernetes ServiceAccount token authenticates as `system:serviceaccount:<namespace>:<name>`, while AccessPolicy members add the `user:` prefix. The dashboard member is therefore `user:system:serviceaccount:agent-substrate-dashboard:agent-substrate-dashboard`. [Agent Substrate authorization guide](https://github.com/agent-substrate/substrate/blob/main/docs/authorization.md#principals)
- Actor observation requires atespace `viewer`; Resume/Suspend requires atespace `editor`. Workers have global scope, and listing workers plus assignments requires global `viewer`. [Agent Substrate authorization guide](https://github.com/agent-substrate/substrate/blob/main/docs/authorization.md#roles)
- Each scope has one policy named `default`. `kubectl ate get/create/update access-policy --global` manages the global policy; `-a "$ATESPACE"` manages the atespace policy. Updates replace the entire policy and require the current `uid` and `version`, so instructions must never blindly overwrite an existing policy. [Agent Substrate authorization guide](https://github.com/agent-substrate/substrate/blob/main/docs/authorization.md#managing-access)
- In the development install, `kubectl ate` defaults to the shared `ate-client` ServiceAccount, which is normally the bootstrap owner, but it warns because that identity is deliberately broad. This is suitable for the copy-paste kind runbook, not production. [Agent Substrate authorization guide](https://github.com/agent-substrate/substrate/blob/main/docs/authorization.md#kubectl-ate-and-ate-setup-identities)

### 5. Recommended installation-guide shape

The product documentation should provide one linear local-kind path first:

1. `cd` to the dashboard checkout.
2. Set and validate `KIND_CLUSTER_NAME`, `KUBE_CONTEXT`, `ATESPACE`, and `WORKER_POOL` in one copyable environment block.
3. Preflight `kubectl`, `kind`, Docker, Substrate's `api` Service, required ClusterTrustBundle, atespace, and WorkerPool.
4. Build and load both `:dev` images.
5. Render a temporary Kustomize overlay using those values rather than editing checked-in YAML.
6. Apply the dashboard and wait for both Deployments.
7. Add the global and atespace AccessPolicy bindings, with separate instructions for a fresh policy versus merging an existing one.
8. Port-forward the frontend Service to `localhost:8090` and open it.
9. Verify `/healthz`, `/readyz`, `/api/v1/meta`, and `/api/v1/snapshot`; show log and event commands for failures.
10. Document upgrade/reinstall (`build`, `kind load`, `rollout restart`) and removal (`kubectl delete -k` or namespace deletion), while stating that removal does not revert AccessPolicy bindings.

For the best copy-paste experience, add a repository-owned `scripts/install-kind.sh` that performs steps 2–6 and prints the policy and port-forward commands. It should not mutate AccessPolicy, preserving the consumer-owned authorization boundary.

## High-priority product corrections revealed by the research

1. Keep `ate.dev/api-access: "true"` on `deploy/kustomize/base/namespace.yaml`.
2. Do not present the hard-coded `demo`/`agents` kind overlay as usable without customization.
3. Prefer port-forwarding over direct NodePort access in macOS/Docker Desktop instructions.
4. Keep preflight validation and the scripted installer so failures occur before Kubernetes reconciliation.

