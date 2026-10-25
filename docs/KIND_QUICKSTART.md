# Install the dashboard on an Agent Substrate kind cluster

This runbook installs the dashboard into an existing kind cluster running Agent Substrate. It deliberately asks for the cluster, atespace, and WorkerPool instead of assuming Herdr-specific names. The dashboard remains observe-only unless you explicitly enable operations.

The commands below build the dashboard locally and load it into kind. This works even before release images are public.

## 1. Check the required tools

You need Docker, `kind`, `kubectl`, the `kubectl ate` plugin, Node.js 22, and npm 10.

```sh
docker version
kind version # must be v0.27.0 or newer
kubectl version --client
kubectl ate version
node --version
npm --version
```

Agent Substrate's development quickstart installs the `kubectl ate` plugin and creates a kind cluster with the feature gates needed for projected identity. If your cluster was created another way, verify that it supports `ClusterTrustBundle` projection before continuing. See the [Agent Substrate quickstart](https://github.com/agent-substrate/substrate#quickstart-development) and [kind image-loading guide](https://kind.sigs.k8s.io/docs/user/quick-start/#loading-an-image-into-your-cluster).

kind v0.26.0 cannot load images into nodes using containerd 2.x and fails with `ERROR: failed to detect containerd snapshotter`. Upgrade to at least v0.27.0 before using the local-image path:

```sh
# macOS with Homebrew
brew upgrade kind
kind version
```

## 2. Open the dashboard checkout

Open a terminal in the repository root and confirm the expected files are present:

```sh
test -f scripts/install-kind.sh
test -f deploy/kustomize/base/kustomization.yaml
```

## 3. Select and verify the kind cluster

List the available kind clusters and contexts:

```sh
kind get clusters
kubectl config get-contexts -o name | grep '^kind-'
```

Paste this block, then enter the kind cluster name when prompted. The normal Agent Substrate default is `kind`, which corresponds to context `kind-kind`.

```sh
read -r -p "kind cluster name [kind]: " KIND_CLUSTER_NAME
export KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-kind}"
export KUBE_CONTEXT="kind-${KIND_CLUSTER_NAME}"
kubectl config use-context "$KUBE_CONTEXT"
kubectl cluster-info
kubectl -n ate-system get service api
kubectl get clustertrustbundles -l podcert.ate.dev/canarying=live
```

Stop here if the `api` Service or live ClusterTrustBundle is missing. The dashboard backend depends on both.

## 4. Select the atespace and WorkerPool

Discover the real values from the running cluster:

```sh
kubectl ate get atespaces
kubectl get workerpools.ate.dev -A
```

Paste this block and enter the exact names shown above:

```sh
read -r -p "dashboard atespace: " DASHBOARD_ATESPACE
read -r -p "dashboard WorkerPool name: " DASHBOARD_WORKER_POOL
export DASHBOARD_ATESPACE DASHBOARD_WORKER_POOL
test -n "$DASHBOARD_ATESPACE"
test -n "$DASHBOARD_WORKER_POOL"
kubectl ate get actors -a "$DASHBOARD_ATESPACE"
kubectl ate get workers
```

An empty dashboard usually means these values do not match the actors and workers you intend to observe. Actors and workers are Control API resources, so verify them with `kubectl ate`, not by looking for actor Pods or CRDs.

## 5. Build, load, and install

Keep operations disabled for the first installation:

```sh
export DASHBOARD_IMAGE_SOURCE=local
export DASHBOARD_ENABLE_OPERATIONS=false
./scripts/install-kind.sh
```

The installer performs preflight checks, builds both images, loads them into the selected kind cluster, generates a local Kustomize overlay with your scope, applies it, and waits for both Deployments. It does not change AccessPolicy.

Confirm that both Pods are running:

```sh
kubectl -n agent-substrate-dashboard get pods
kubectl -n agent-substrate-dashboard get services
```

Both Services should remain `ClusterIP`. Browser access is established with port-forwarding in step 7; the backend is never exposed directly.

### Use published images instead

If the GHCR packages are public or your cluster has suitable pull credentials, skip the local build and kind image load:

```sh
export DASHBOARD_IMAGE_SOURCE=ghcr
export DASHBOARD_IMAGE_OWNER=agent-substrate
export DASHBOARD_IMAGE_TAG=v0.1.0
export DASHBOARD_ENABLE_OPERATIONS=false
./scripts/install-kind.sh
```

## 6. Grant observe-only access

The dashboard ServiceAccount needs global `viewer` to list workers and assignments, plus atespace `viewer` to list actors. AccessPolicy is a singleton in each scope, and an update replaces the complete document. The preparation command below fetches the current policies, preserves their metadata and other bindings, and writes reviewable candidates. It never applies them.

```sh
npm ci
node scripts/prepare-access-policies.mjs \
  --atespace "$DASHBOARD_ATESPACE" \
  --role viewer
sed -n '1,240p' .cache/dashboard-access/global.yaml
sed -n '1,240p' .cache/dashboard-access/atespace.yaml
```

Review both files. The preparation command prints the exact `kubectl ate create` or `update` commands needed for this cluster; copy and run those printed commands only after the files look correct.

Verify the resulting policies:

```sh
kubectl ate get access-policy --global -o yaml
kubectl ate get access-policy -a "$DASHBOARD_ATESPACE" -o yaml
```

The expected member is:

```text
user:system:serviceaccount:agent-substrate-dashboard:agent-substrate-dashboard
```

See Agent Substrate's [authorization guide](https://github.com/agent-substrate/substrate/blob/main/docs/authorization.md#managing-access) for the role model and replacement semantics.

## 7. Open and verify the dashboard

Run this command in a terminal and leave it running:

```sh
kubectl -n agent-substrate-dashboard \
  port-forward service/agent-substrate-dashboard-frontend 8090:80
```

In a second terminal:

```sh
curl -fsS http://127.0.0.1:8090/healthz
curl -fsS http://127.0.0.1:8090/readyz
curl -fsS http://127.0.0.1:8090/api/v1/meta
curl -fsS http://127.0.0.1:8090/api/v1/snapshot
open http://127.0.0.1:8090
```

On Linux, replace `open` with `xdg-open`. Port-forwarding is used because a kind NodePort is not reliably reachable from the host unless the cluster was created with a matching `extraPortMappings` entry.

## 8. Optional: enable Resume and Suspend

Operation mode requires both an atespace `editor` binding and explicit backend configuration. Prepare and review the editor policy first:

```sh
node scripts/prepare-access-policies.mjs \
  --atespace "$DASHBOARD_ATESPACE" \
  --role editor
sed -n '1,240p' .cache/dashboard-access/atespace.yaml
```

Run the printed policy command after review, then reinstall with operations enabled:

```sh
export DASHBOARD_ENABLE_OPERATIONS=true
./scripts/install-kind.sh
```

The global role remains `viewer`; workers are observed but never mutated by this dashboard.

## Troubleshooting

Collect the basic state first:

```sh
kubectl -n agent-substrate-dashboard get pods,services,networkpolicies
kubectl -n agent-substrate-dashboard get events --sort-by=.lastTimestamp
kubectl -n agent-substrate-dashboard logs deployment/agent-substrate-dashboard-backend
kubectl -n agent-substrate-dashboard logs deployment/agent-substrate-dashboard-frontend
kubectl -n agent-substrate-dashboard describe pod -l app.kubernetes.io/component=backend
```

- `ImagePullBackOff`: for local images, confirm `KIND_CLUSTER_NAME` names the cluster you loaded. For GHCR, confirm the package is public or configure an image pull secret.
- `failed to detect containerd snapshotter`: `kind version` is older than v0.27.0. Upgrade kind and rerun the installer; recreating the cluster is not required for this loader error.
- Backend connection timeout: verify `kubectl get namespace agent-substrate-dashboard --show-labels` includes `ate.dev/api-access=true`, and verify the `api` Service exists in `ate-system`.
- Projected-volume or ClusterTrustBundle error: the kind cluster was not created with the Agent Substrate feature gates; recreate it using the supported Substrate cluster setup.
- `PermissionDenied` from `/api/v1/snapshot`: verify both the global and atespace policies and the exact ServiceAccount member shown above.
- Dashboard is healthy but empty: repeat step 4 and confirm the selected atespace and WorkerPool contain the actors and workers shown by `kubectl ate`.
- Frontend works but `/api/v1/*` returns `502`: inspect backend logs and readiness; the frontend proxy is working but the backend is unavailable.

## Upgrade a local installation

Re-run the same installation command after changing the source. It rebuilds and reloads both `:dev` images. Restart the Deployments so every Pod uses the newly loaded image:

```sh
./scripts/install-kind.sh
kubectl -n agent-substrate-dashboard rollout restart \
  deployment/agent-substrate-dashboard-backend \
  deployment/agent-substrate-dashboard-frontend
kubectl -n agent-substrate-dashboard rollout status \
  deployment/agent-substrate-dashboard-backend --timeout=180s
kubectl -n agent-substrate-dashboard rollout status \
  deployment/agent-substrate-dashboard-frontend --timeout=180s
```

## Remove the dashboard

```sh
kubectl delete namespace agent-substrate-dashboard
```

This removes both dashboard components and their Kubernetes resources. It does not remove the two Agent Substrate AccessPolicy bindings; policy cleanup must be reviewed and performed separately so unrelated bindings are never overwritten.
