# Security Boundary

[简体中文](security.zh-CN.md)

Driver modules receive non-secret configuration, endpoint reference names,
credential slot names with configured booleans, exact protocol JSON, and the
selected upstream model. They never receive credential values and never perform
network I/O.

Core validates request and response contracts, RequestPlan restrictions,
resource budgets, headers, URLs, status codes, SSE/NDJSON framing and event
state. Core also owns credentials, transport, timeouts, routing, failover,
breakers and analytics. Driver errors and vendor codes must contain only
bounded, secret-safe diagnostics.

For Image calls, file and large upstream-response bytes remain in
attempt-scoped Host spools. The Guest receives only `BlobRef` metadata and has
no API for opening a blob or discovering its path. A Driver may reference a
blob in a bounded BodyPlan, including standard padded Base64. Core validates
ownership, length, digest, expanded length, headers, and JSON before network
I/O or caller response commit. Blob references expire when the attempt closes.
