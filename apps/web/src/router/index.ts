import { createRouter, createWebHistory } from "vue-router";
import type { RouteRecordRaw } from "vue-router";

const routes: RouteRecordRaw[] = [
  {
    path: "/",
    component: () => import("../layouts/MainLayout.vue"),
    children: [
      {
        path: "",
        name: "projects",
        component: () => import("../pages/ProjectsPage.vue"),
      },
    ],
  },
  {
    path: "/auth",
    component: () => import("../layouts/AuthLayout.vue"),
    children: [
      {
        path: "sign-in",
        name: "sign-in",
        component: () => import("../pages/AuthPage.vue"),
      },
    ],
  },
  {
    path: "/:catchAll(.*)*",
    component: () => import("../pages/NotFoundPage.vue"),
  },
];

export default createRouter({
  history: createWebHistory(),
  routes,
});
