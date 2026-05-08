import { Outlet } from "@tanstack/react-router";

export function AuthLayout() {
  return (
    <main className="flex min-h-screen items-center justify-center bg-muted/40">
      <Outlet />
    </main>
  );
}
