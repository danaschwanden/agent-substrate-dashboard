# Keep dashboard commands native to Agent Substrate

The dashboard will replace the reference implementation’s OpenClaw-specific burst and named-fleet churn commands with bulk Resume and Suspend operations over existing actors. Workload generation and application health probes remain outside the dashboard, so its control surface works across Agent Substrate implementations.
