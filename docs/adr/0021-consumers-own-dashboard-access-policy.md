# Let consuming deployments grant dashboard access

Each consuming Agent Substrate implementation will explicitly grant the dashboard ServiceAccount viewer or editor access to the configured atespace. The Dashboard Backend will never create, modify, or widen its own AccessPolicy; the portable repository will document required permissions and fail clearly when they are absent.
