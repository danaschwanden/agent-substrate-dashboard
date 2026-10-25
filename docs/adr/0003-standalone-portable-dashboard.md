# Build a standalone portable dashboard

The Dashboard Frontend and Dashboard Backend will live in a new repository and integrate with Agent Substrate implementations through explicit contracts. The `always-on-agent` dashboard is a visual and behavioral reference only: the new project will not carry its OpenClaw-specific source, deployment structure, or assumptions, allowing the dashboard to be added to implementations such as `herdr-agent-substrate-on-kind` without coupling their application code.
