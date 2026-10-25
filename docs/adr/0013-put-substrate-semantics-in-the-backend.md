# Put Agent Substrate semantics in the backend

The Dashboard Backend will paginate and normalize Control API resources, join actors to workers, infer observed transitions, and calculate fleet metrics before returning its versioned HTTP model. The Dashboard Frontend will present that model without interpreting protobuf enums or reconstructing Substrate relationships.
