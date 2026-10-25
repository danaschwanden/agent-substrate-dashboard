# Ship a Kustomize base

The dashboard repository will publish a Kustomize base for the independent frontend and backend Deployments, Services, ServiceAccount, and configuration. Each Agent Substrate implementation will own a small overlay for image digests, endpoints, exposure, and local policy, avoiding an additional package-manager dependency or duplicate Helm packaging in the first release.
