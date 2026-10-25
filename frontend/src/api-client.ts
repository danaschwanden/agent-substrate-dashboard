import type { BulkOperationResponse, DashboardSnapshot, Meta } from "./types.js";

export class DashboardApiClient {
  constructor(private readonly baseUrl = "/api/v1") {}

  readMeta(signal?: AbortSignal): Promise<Meta> {
    return this.readJson<Meta>(`${this.baseUrl}/meta`, { signal });
  }

  readSnapshot(signal?: AbortSignal): Promise<DashboardSnapshot> {
    return this.readJson<DashboardSnapshot>(`${this.baseUrl}/snapshot`, { signal });
  }

  operate(operation: "resume" | "suspend", actorNames: string[], signal?: AbortSignal): Promise<BulkOperationResponse> {
    return this.readJson<BulkOperationResponse>(`${this.baseUrl}/operations/${operation}`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ actorNames }),
      signal,
    });
  }

  private async readJson<T>(url: string, init?: RequestInit): Promise<T> {
    const response = await fetch(url, init);
    if (!response.ok) {
      const fallback = `${response.status} ${response.statusText}`.trim();
      const body = await response.json().catch(() => undefined) as { message?: string } | undefined;
      throw new Error(body?.message ?? fallback);
    }
    return response.json() as Promise<T>;
  }
}
