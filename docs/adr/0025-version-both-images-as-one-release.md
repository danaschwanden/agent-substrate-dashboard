# Version both component images as one release

Each repository tag will publish matching Dashboard Frontend and Dashboard Backend images plus the corresponding Kustomize base under one version. Deployments may roll the images independently, but the release identifies the intended compatible pair and the frontend still rejects unsupported backend API versions.
