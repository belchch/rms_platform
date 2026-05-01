import { create } from "zustand";
import { persist } from "zustand/middleware";

import { apiClient } from "@/api/client";

export interface AuthState {
  accessToken: string | null;
  refreshToken: string | null;
  signIn: (email: string, password: string) => Promise<void>;
  signOut: () => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      accessToken: null,
      refreshToken: null,
      signIn: async (email: string, password: string) => {
        const response = await apiClient.signIn({ email, password });
        set({
          accessToken: response.accessToken,
          refreshToken: response.refreshToken,
        });
      },
      signOut: () => set({ accessToken: null, refreshToken: null }),
    }),
    { name: "rms-auth" },
  ),
);
