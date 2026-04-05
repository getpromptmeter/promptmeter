import type { ApiResponse, ApiError } from "./types";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

class ApiClient {
  private baseUrl: string;
  private refreshing: Promise<boolean> | null = null;

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl;
  }

  async get<T>(path: string, params?: Record<string, string>): Promise<T> {
    const url = new URL(path, this.baseUrl);
    if (params) {
      Object.entries(params).forEach(([k, v]) => {
        if (v) url.searchParams.set(k, v);
      });
    }
    return this.request<T>(url.toString(), { method: "GET" });
  }

  async post<T>(path: string, body?: unknown): Promise<T> {
    const url = new URL(path, this.baseUrl);
    return this.request<T>(url.toString(), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: body ? JSON.stringify(body) : undefined,
    });
  }

  async put<T>(path: string, body?: unknown): Promise<T> {
    const url = new URL(path, this.baseUrl);
    return this.request<T>(url.toString(), {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: body ? JSON.stringify(body) : undefined,
    });
  }

  async delete<T>(path: string): Promise<T> {
    const url = new URL(path, this.baseUrl);
    return this.request<T>(url.toString(), { method: "DELETE" });
  }

  private async request<T>(url: string, init: RequestInit): Promise<T> {
    const response = await fetch(url, {
      ...init,
      credentials: "include",
    });

    // Handle 401 -- try refresh
    if (response.status === 401) {
      const refreshed = await this.tryRefresh();
      if (refreshed) {
        const retryResponse = await fetch(url, {
          ...init,
          credentials: "include",
        });
        return this.handleResponse<T>(retryResponse);
      }
      // Refresh failed, redirect to login
      if (typeof window !== "undefined") {
        window.location.href = "/login";
      }
      throw new Error("Authentication required");
    }

    return this.handleResponse<T>(response);
  }

  private async handleResponse<T>(response: Response): Promise<T> {
    const json = await response.json();

    if (!response.ok) {
      const error = json as ApiError;
      throw new ApiClientError(
        error.error?.message || "Request failed",
        error.error?.code || "UNKNOWN",
        response.status
      );
    }

    // Unwrap envelope -- return data directly
    const envelope = json as ApiResponse<T>;
    return envelope.data;
  }

  private async tryRefresh(): Promise<boolean> {
    // Deduplicate concurrent refresh attempts
    if (this.refreshing) {
      return this.refreshing;
    }

    this.refreshing = (async () => {
      try {
        const response = await fetch(
          new URL("/api/v1/auth/refresh", this.baseUrl).toString(),
          {
            method: "POST",
            credentials: "include",
          }
        );
        return response.ok;
      } catch {
        return false;
      } finally {
        this.refreshing = null;
      }
    })();

    return this.refreshing;
  }
}

export class ApiClientError extends Error {
  code: string;
  status: number;

  constructor(message: string, code: string, status: number) {
    super(message);
    this.name = "ApiClientError";
    this.code = code;
    this.status = status;
  }
}

export const api = new ApiClient(API_BASE);
