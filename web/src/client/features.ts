import { get } from "./http";
import type { FilterCondition } from "./types";

export interface OgcFeature {
  type: "Feature";
  id: string;
  geometry: GeoJSON.Geometry | null;
  properties: Record<string, unknown>;
}

export interface OgcLink {
  rel: string;
  type: string;
  href: string;
}

export interface OgcFeatureCollection {
  type: "FeatureCollection";
  features: OgcFeature[];
  numberMatched: number;
  numberReturned: number;
  links: OgcLink[];
}

export interface FeatureQueryOptions {
  bbox?: [number, number, number, number];
  filter?: FilterCondition[];
  simplify?: number;
  limit?: number;
  offset?: number;
}

export function featureQueryParams(opts: FeatureQueryOptions): URLSearchParams {
  const params = new URLSearchParams();
  if (opts.bbox) params.set("bbox", opts.bbox.join(","));
  if (opts.filter?.length) params.set("filter", JSON.stringify(opts.filter));
  if (opts.simplify !== undefined) params.set("simplify", String(opts.simplify));
  if (opts.limit !== undefined) params.set("limit", String(opts.limit));
  if (opts.offset !== undefined) params.set("offset", String(opts.offset));
  return params;
}

export function getItems(tableId: string, opts: FeatureQueryOptions = {}): Promise<OgcFeatureCollection> {
  const qs = featureQueryParams(opts).toString();
  return get(`/ogc/collections/${tableId}/items${qs ? `?${qs}` : ""}`);
}
