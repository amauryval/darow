import { get, post, postForm } from "./http";
import type { Source } from "./types";

export function listSources(): Promise<Source[]> {
  return get("/sources");
}

export function refreshSources(): Promise<Source[]> {
  return post("/sources/refresh");
}

// layer is required for a multi-layer (.gpkg) source — omit it otherwise.
export function loadSource(name: string, layer?: string): Promise<{ tableId: string }> {
  const query = layer ? `?layer=${encodeURIComponent(layer)}` : "";
  return post(`/sources/${encodeURIComponent(name)}/load${query}`);
}

// Uploads file into workDir and immediately loads it into a new table, in
// one round trip (POST /sources/upload) — unless the file is a GeoPackage
// with more than one layer, in which case no layer is auto-selected and the
// refreshed Source (with its layers) is returned instead, so the caller can
// load a specific layer via loadSource.
export function uploadSource(file: File): Promise<{ tableId: string } | Source> {
  const form = new FormData();
  form.append("file", file);
  return postForm("/sources/upload", form);
}
