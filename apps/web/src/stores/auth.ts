import { defineStore } from "pinia";
import { ref } from "vue";
import { apiClient } from "../api/client";

export const useAuthStore = defineStore("auth", () => {
  const accessToken = ref<string | null>(null);
  const isAuthenticated = ref(false);

  async function signIn(email: string, password: string): Promise<void> {
    const response = await apiClient.signIn({ email, password });
    accessToken.value = response.accessToken;
    isAuthenticated.value = true;
  }

  function signOut(): void {
    accessToken.value = null;
    isAuthenticated.value = false;
  }

  return { accessToken, isAuthenticated, signIn, signOut };
});
