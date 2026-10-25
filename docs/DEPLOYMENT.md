# Deploying the Agent Substrate Dashboard

The dashboard ships as two independently built images and a Kustomize base. The frontend is the only browser-facing service; it serves the Lit application and proxies `/api/v1/*` to the cluster-internal backend.

## Build the images

Build both release images for the supported architectures:

```sh
docker buildx build --platform linux/amd64,linux/arm64 \
  --build-arg VERSION=v0.1.0 \
  -f backend/Dockerfile -t REGISTRY/agent-substrate-dashboard-backend:v0.1.0 .

docker buildx build --platform linux/amd64,linux/arm64 \
  --build-arg VERSION=v0.1.0 \
  -f frontend/Dockerfile -t REGISTRY/agent-substrate-dashboard-frontend:v0.1.0 .
```

Add `--push` only when intentionally publishing locally. Pull-request CI builds both platforms without publishing them.

## Published images

The `Publish container images` GitHub workflow publishes both multi-architecture images to GitHub Container Registry on:

- pushes to `main`;
- tags beginning with `v`, such as `v0.1.0`;
- manual workflow runs.

Each publication receives an immutable `sha-<commit>` tag. Branch pushes also receive the branch name, the default branch receives `latest`, and version tags retain their complete tag such as `v0.1.0`:

```text
ghcr.io/OWNER/agent-substrate-dashboard-backend:TAG
ghcr.io/OWNER/agent-substrate-dashboard-frontend:TAG
```

The workflow uses the repository-scoped `GITHUB_TOKEN`; no registry password is required. Its publish job has `packages: write`, while the rest of the workflow remains read-only. Package visibility and access are managed in the repository or organization’s GitHub Packages settings.

## Configure an overlay

Start with [`deploy/kustomize/base`](../deploy/kustomize/base). A consumer overlay must set:

- the configured atespace and worker pool;
- the Control API endpoint and TLS server name if they differ from `api.ate-system.svc:443` and `api.ate-system.svc`;
- both image references, preferably by digest;
- the frontend exposure appropriate for the trusted operator network.

The [kind overlay](../deploy/kustomize/overlays/kind) is a concrete development example. It exposes only the frontend through NodePort `30090`; the backend remains `ClusterIP` and a NetworkPolicy accepts backend traffic only from frontend pods.

The [digest-pinned example](../deploy/kustomize/examples/digest-pinned) demonstrates the Kustomize `images[].digest` shape. Its all-zero values are placeholders and must be replaced before deployment:

```yaml
images:
- name: ghcr.io/danaschwanden/agent-substrate-dashboard-backend
  newName: registry.example/agent-substrate-dashboard-backend
  digest: sha256:REPLACE_WITH_BACKEND_DIGEST
- name: ghcr.io/danaschwanden/agent-substrate-dashboard-frontend
  newName: registry.example/agent-substrate-dashboard-frontend
  digest: sha256:REPLACE_WITH_FRONTEND_DIGEST
```

Render before applying:

```sh
kubectl kustomize deploy/kustomize/overlays/kind
kubectl apply -k deploy/kustomize/overlays/kind
```

## Projected identity

The backend does not use ambient Kubernetes API credentials. The base mounts:

- a rotating ServiceAccount token with audience `api.ate-system.svc`;
- the live `servicedns.podcert.ate.dev/identity` ClusterTrustBundle.

The Agent Substrate API server must trust the Kubernetes JWT issuer and the same audience. If an installation uses a different JWT audience, patch both the projected token audience and the API server authentication configuration. If it uses a different serving signer, patch the ClusterTrustBundle projection and `DASHBOARD_CONTROL_TRUST_BUNDLE_FILE` together.

The dashboard ServiceAccount principal in the base is:

```text
user:system:serviceaccount:agent-substrate-dashboard:agent-substrate-dashboard
```

Changing the namespace or ServiceAccount changes this principal.

## AccessPolicy requirements

The dashboard never creates or updates Agent Substrate AccessPolicy resources. A deployment owner must merge the dashboard principal into the installation’s existing policies.

Observation needs two grants because workers are global resources:

1. `viewer` in the configured atespace, to list actors;
2. global `viewer`, to list workers and worker assignments.

Operation mode additionally needs `editor` in the configured atespace for Resume and Suspend. Global `viewer` is still required for the worker panels.

Example binding fragments:

```yaml
# Global AccessPolicy: required for both observation and operation mode.
- role: viewer
  members:
  - user:system:serviceaccount:agent-substrate-dashboard:agent-substrate-dashboard

# Configured atespace AccessPolicy: observe-only.
- role: viewer
  members:
  - user:system:serviceaccount:agent-substrate-dashboard:agent-substrate-dashboard

# Configured atespace AccessPolicy: operation mode; use instead of viewer.
- role: editor
  members:
  - user:system:serviceaccount:agent-substrate-dashboard:agent-substrate-dashboard
```

AccessPolicy is a singleton per scope and updates replace the full document. Fetch the current policy, preserve its `uid`, `version`, and existing bindings, merge the dashboard binding, then update it using the installation’s authorized administrative identity. Do not blindly apply these fragments.

Finally, set `DASHBOARD_ENABLE_OPERATIONS=true` in the consumer overlay. An editor grant alone does not enable controls, and enabling controls without the editor grant produces explicit per-actor permission failures.

## Runtime security

Both containers run as non-root, drop Linux capabilities, disable privilege escalation, and use read-only root filesystems. The base applies the restricted Pod Security profile and gives the frontend only bounded `emptyDir` mounts needed for NGINX’s generated configuration and temporary files.

This first release remains a trusted-network operator surface and has no application-level login. Restrict frontend exposure accordingly.
