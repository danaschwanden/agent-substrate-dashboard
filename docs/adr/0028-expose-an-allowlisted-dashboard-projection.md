# Expose an allowlisted dashboard projection

The Dashboard Backend will return only fields required by the versioned dashboard contract, including resource names, lifecycle states, worker assignments, capacity and allocation, timestamps, and bounded failure summaries. It will not forward raw Agent Substrate protobuf resources or arbitrary labels to the browser.
