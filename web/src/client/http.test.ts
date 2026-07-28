import { describe, expect, it, vi } from "vitest";
import { apiUrl, ApiError, del, get, post } from "./http";

function jsonResponse(body: unknown, status: number): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("http client", () => {
  it("resolves with the parsed body on a 2xx response", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse({ tableId: "t1" }, 200)));

    await expect(get<{ tableId: string }>("/tables/t1")).resolves.toEqual({ tableId: "t1" });
  });

  it("resolves with undefined on a 204 response", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(null, { status: 204 })));

    await expect(del<void>("/tables/t1/rows/pk-1")).resolves.toBeUndefined();
  });

  it("rejects with an ApiError carrying the server's error body on failure", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(jsonResponse({ error: "not found", message: "Not Found", code: 404 }, 404)),
    );

    await expect(get("/tables/unknown")).rejects.toMatchObject({ message: "not found", code: 404 });
  });

  it("falls back to the status text when the error body isn't JSON", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("boom", { status: 500, statusText: "Internal Server Error" })));

    await expect(get("/tables")).rejects.toBeInstanceOf(ApiError);
  });

  it("sends a JSON content-type and body on post", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ ok: true }, 201));
    vi.stubGlobal("fetch", fetchMock);

    await post("/tables/t1/derive", { query: "SELECT 1" });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/tables/t1/derive",
      expect.objectContaining({
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ query: "SELECT 1" }),
      }),
    );
  });

  it("prefixes every request with /api/v1", () => {
    expect(apiUrl("/tables/t1")).toBe("/api/v1/tables/t1");
  });
});
