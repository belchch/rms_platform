import { defineConfig } from "#q-app/wrappers";

export default defineConfig((/* ctx */) => {
  return {
    boot: ["pinia"],

    css: ["app.scss"],

    extras: ["roboto-font", "material-icons"],

    build: {
      target: {
        browser: ["es2022"],
        node: "node20",
      },
      vueRouterMode: "history",
      typescript: {
        strict: true,
      },
    },

    devServer: {
      port: 3000,
      proxy: {
        "/api": {
          target: "http://localhost:8080",
          changeOrigin: true,
        },
        "/health": {
          target: "http://localhost:8080",
          changeOrigin: true,
        },
      },
    },

    framework: {
      config: {},
      plugins: ["Notify", "Dialog", "Loading"],
    },

    animations: [],

    ssr: {
      pwa: false,
      prodPort: 3000,
      middlewares: ["render"],
    },

    pwa: {
      workboxMode: "GenerateSW",
    },
  };
});
