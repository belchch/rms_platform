import {
  createRootRoute,
  createRoute,
  createRouter,
  redirect,
} from "@tanstack/react-router";

import { App } from "@/App";
import { AuthLayout } from "@/layouts/AuthLayout";
import { MainLayout } from "@/layouts/MainLayout";
import { AuthPage } from "@/pages/AuthPage";
import { NotFoundPage } from "@/pages/NotFoundPage";
import { ProjectsPage } from "@/pages/ProjectsPage";
import { useAuthStore } from "@/stores/auth";

const rootRoute = createRootRoute({
  component: App,
});

const authenticatedRoute = createRoute({
  id: "_authenticated",
  getParentRoute: () => rootRoute,
  component: MainLayout,
  beforeLoad: ({ location }) => {
    if (!useAuthStore.getState().accessToken) {
      throw redirect({
        to: "/auth/sign-in",
        search: {
          redirect: location.pathname,
        },
      });
    }
  },
});

const indexRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: "/",
  component: ProjectsPage,
});

const authLayoutRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/auth",
  component: AuthLayout,
});

const signInRoute = createRoute({
  getParentRoute: () => authLayoutRoute,
  path: "/sign-in",
  validateSearch: (search: Record<string, unknown>): { redirect?: string } => ({
    redirect:
      typeof search.redirect === "string" && search.redirect.startsWith("/")
        ? search.redirect
        : undefined,
  }),
  component: AuthPage,
});

const notFoundRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "$",
  component: NotFoundPage,
});

const routeTree = rootRoute.addChildren([
  authenticatedRoute.addChildren([indexRoute]),
  authLayoutRoute.addChildren([signInRoute]),
  notFoundRoute,
]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
