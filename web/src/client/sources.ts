import { get, post, postForm } from "./http";
import type { Source } from "./types";

export function listSources(): Promise<Source[]> {
  return get("/sources");
}

export function refreshSources(): Promise<Source[]> {
  return post("/sources/refresh");
}

export function loadSource(name: string): Promise<{ tableId: string }> {
  return post(`/sources/${encodeURIComponent(name)}/load`);
}

// Uploads file into workDir and immediately loads it into a new table, in
// one round trip (POST /sources/upload).
export function uploadSource(file: File): Promise<{ tableId: string }> {
  const form = new FormData();
  form.append("file", file);
  return postForm("/sources/upload", form);
}
