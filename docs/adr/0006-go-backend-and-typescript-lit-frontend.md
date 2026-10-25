# Use Go for the backend and TypeScript with Lit for the frontend

The Dashboard Backend will use Go so it can consume Agent Substrate’s generated protobuf client directly and ship as a small standalone binary. The independently built Dashboard Frontend will use TypeScript and Lit, prioritizing the simplest typed implementation within each component over sharing one language across the deployment boundary.
