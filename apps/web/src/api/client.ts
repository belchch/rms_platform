import type { paths } from "@rms/api-contracts/types.gen";

type SignInRequest =
  paths["/api/v1/auth/sign-in"]["post"]["requestBody"]["content"]["application/json"];

type TokenPair =
  paths["/api/v1/auth/sign-in"]["post"]["responses"]["200"]["content"]["application/json"];

type RefreshRequest =
  paths["/api/v1/auth/refresh"]["post"]["requestBody"]["content"]["application/json"];

type PullResponse =
  paths["/api/v1/sync/pull"]["get"]["responses"]["200"]["content"]["application/json"];

type PushRequest =
  paths["/api/v1/sync/push"]["post"]["requestBody"]["content"]["application/json"];

type PushResponse =
  paths["/api/v1/sync/push"]["post"]["responses"]["200"]["content"]["application/json"];

type PhotoUploadResponse =
  paths["/api/v1/photos"]["post"]["responses"]["201"]["content"]["application/json"];

const BASE_URL = "";

async function request<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const response = await fetch(`${BASE_URL}${path}`, {
    headers: {
      "Content-Type": "application/json",
      ...options.headers,
    },
    ...options,
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ message: response.statusText }));
    throw new Error(error.message ?? "Request failed");
  }

  return response.json() as Promise<T>;
}

function authHeaders(token: string): HeadersInit {
  return { Authorization: `Bearer ${token}` };
}

export const apiClient = {
  async signIn(body: SignInRequest): Promise<TokenPair> {
    return request<TokenPair>("/api/v1/auth/sign-in", {
      method: "POST",
      body: JSON.stringify(body),
    });
  },

  async refreshToken(body: RefreshRequest): Promise<TokenPair> {
    return request<TokenPair>("/api/v1/auth/refresh", {
      method: "POST",
      body: JSON.stringify(body),
    });
  },

  async syncPull(token: string, since: number = 0): Promise<PullResponse> {
    return request<PullResponse>(`/api/v1/sync/pull?since=${since}`, {
      headers: authHeaders(token),
    });
  },

  async syncPush(token: string, body: PushRequest): Promise<PushResponse> {
    return request<PushResponse>("/api/v1/sync/push", {
      method: "POST",
      headers: authHeaders(token),
      body: JSON.stringify(body),
    });
  },

  async uploadPhoto(token: string, file: File): Promise<PhotoUploadResponse> {
    const formData = new FormData();
    formData.append("file", file);
    const response = await fetch("/api/v1/photos", {
      method: "POST",
      headers: authHeaders(token),
      body: formData,
    });
    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: response.statusText }));
      throw new Error(error.message ?? "Upload failed");
    }
    return response.json() as Promise<PhotoUploadResponse>;
  },
};
