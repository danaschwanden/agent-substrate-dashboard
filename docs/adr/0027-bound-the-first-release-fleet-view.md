# Bound the first-release fleet view

The first release will target scopes of up to 500 actors and 100 workers with a single polled Dashboard Snapshot. The backend will still paginate the native Control API, enforce configurable presentation limits, and report truncation explicitly; larger interactive fleets will require a later server-side query and virtualization design.
