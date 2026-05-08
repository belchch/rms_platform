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

type PhotoUploadUrlRequest =
  paths["/api/v1/photos/upload-url"]["post"]["requestBody"]["content"]["application/json"];

type PhotoUploadUrlResponse =
  paths["/api/v1/photos/upload-url"]["post"]["responses"]["200"]["content"]["application/json"];

const BASE_URL = "";

async function requestJson<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const response = await fetch(`${BASE_URL}${path}`, {
    headers: {
      "Content-Type": "application/json",
      ...options.headers,
    },
    ...options,
  });

  if (!response.ok) {
    const raw = await response.json().catch(() => null);
    let message = response.statusText;
    if (
      raw &&
      typeof raw === "object" &&
      "detail" in raw &&
      typeof (raw as { detail?: unknown }).detail === "string"
    ) {
      message = (raw as { detail: string }).detail;
    } else if (
      raw &&
      typeof raw === "object" &&
      "message" in raw &&
      typeof (raw as { message?: unknown }).message === "string"
    ) {
      message = (raw as { message: string }).message;
    }
    throw new Error(message || "Request failed");
  }

  return response.json() as Promise<T>;
}

function authHeaders(token: string): HeadersInit {
  return { Authorization: `Bearer ${token}` };
}

export const apiClient = {
  async signIn(body: SignInRequest): Promise<TokenPair> {
    return requestJson<TokenPair>("/api/v1/auth/sign-in", {
      method: "POST",
      body: JSON.stringify(body),
    });
  },

  async refreshToken(body: RefreshRequest): Promise<TokenPair> {
    return requestJson<TokenPair>("/api/v1/auth/refresh", {
      method: "POST",
      body: JSON.stringify(body),
    });
  },

  async syncPull(token: string, since = 0): Promise<PullResponse> {
    return requestJson<PullResponse>(`/api/v1/sync/pull?since=${since}`, {
      headers: authHeaders(token),
    });
  },

  async syncPush(token: string, body: PushRequest): Promise<PushResponse> {
    return requestJson<PushResponse>("/api/v1/sync/push", {
      method: "POST",
      headers: authHeaders(token),
      body: JSON.stringify(body),
    });
  },

  async requestPhotoUploadUrl(
    token: string,
    body: PhotoUploadUrlRequest,
  ): Promise<PhotoUploadUrlResponse> {
    return requestJson<PhotoUploadUrlResponse>("/api/v1/photos/upload-url", {
      method: "POST",
      headers: authHeaders(token),
      body: JSON.stringify(body),
    });
  },
};
