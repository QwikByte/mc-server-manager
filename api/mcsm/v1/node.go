package mcsmv1

// RuntimeUnavailable is the runtime that GetInfo reports while the agent can't reach it.
const RuntimeUnavailable = "unavailable"

// ReasonRuntimeUnavailable is the reason of the ErrorInfo that agents attach to Unavailable
// errors of calls that failed because the container runtime can't be reached.
const ReasonRuntimeUnavailable = "RUNTIME_UNAVAILABLE"
