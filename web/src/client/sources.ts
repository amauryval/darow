import { del, get, post, postForm } from "./http";
import type { Source, SourceCRS } from "./types";

export function listSources(): Promise<Source[]> {
  return get("/sources");
}

export function refreshSources(): Promise<Source[]> {
  return post("/sources/refresh");
}

// layer is required for a multi-layer (.gpkg) source — omit it otherwise.
// srid pins the source CRS explicitly, overriding auto-detection —
// required if getSourceMetadata reports a null srid.
export function loadSource(name: string, layer?: string, srid?: number): Promise<{ tableId: string }> {
  const params = new URLSearchParams();
  if (layer) params.set("layer", layer);
  if (srid) params.set("srid", String(srid));
  const query = params.toString() ? `?${params.toString()}` : "";
  return post(`/sources/${encodeURIComponent(name)}/load${query}`);
}

// Previews a not-yet-loaded source's detected CRS, so the caller can show
// it to the user (with an editable override) before Load.
export function getSourceMetadata(name: string, layer?: string): Promise<SourceCRS> {
  const query = layer ? `?layer=${encodeURIComponent(layer)}` : "";
  return get(`/sources/${encodeURIComponent(name)}/metadata${query}`);
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

// Removes the source file from workDir. If it (or one of its layers) was
// loaded, the associated table(s) are removed too. Explicit replacement is
// delete then re-upload.
export function deleteSource(name: string): Promise<void> {
  return del(`/sources/${encodeURIComponent(name)}`);
}
