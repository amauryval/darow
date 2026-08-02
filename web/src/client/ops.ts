import { getRoot } from "./http";

// Liveness probe — resolves if the process is alive, regardless of storage
// state; rejects (via ApiError/network error) otherwise.
export function healthz(): Promise<void> {
  return getRoot("/healthz");
}

// Readiness probe — resolves if the server can currently serve traffic
// (storage reachable); rejects with an ApiError on 503 otherwise.
export function readyz(): Promise<void> {
  return getRoot("/readyz");
}
