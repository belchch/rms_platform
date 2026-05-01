import { Outlet, useNavigate } from "@tanstack/react-router";

import { Button } from "@/components/ui/button";
import { useAuthStore } from "@/stores/auth";

export function MainLayout() {
  const navigate = useNavigate();
  const signOut = useAuthStore((s) => s.signOut);

  return (
    <div className="flex min-h-screen flex-col">
      <header className="border-b bg-background px-6 py-4">
        <div className="flex items-center justify-between">
          <h1 className="text-lg font-semibold">RMS Platform</h1>
          <Button
            type="button"
            variant="outline"
            onClick={() => {
              signOut();
              void navigate({ to: "/auth/sign-in" });
            }}
          >
            Выйти
          </Button>
        </div>
      </header>
      <main className="container mx-auto flex-1 p-6">
        <Outlet />
      </main>
    </div>
  );
}
