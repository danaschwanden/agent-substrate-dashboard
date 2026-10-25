# Expose only the frontend service to browsers

The frontend service will serve the Lit assets and proxy `/api/v1/*` to the cluster-internal Dashboard Backend service. This preserves independently built and deployed components while avoiding browser CORS configuration and keeping the backend off the external network path.
