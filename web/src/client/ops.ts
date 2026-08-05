import { getRoot } from "./http";

// Readiness probe — resolves if the server can currently serve traffic
// (storage reachable); rejects with an ApiError on 503 otherwise.
export function readyz(): Promise<void> {
  return getRoot("/readyz");
}
