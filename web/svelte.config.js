import adapter from "@sveltejs/adapter-static";
import { vitePreprocess } from "@sveltejs/vite-plugin-svelte";

// A static single-page app: there is no Node server -- the Go binary embeds
// the build output (internal/webui/embed.go) and serves index.html for any
// path that isn't a real file, so client-side routing handles deep links.
// appDir "assets" keeps the hashed bundle under assets/, which embed.go
// caches as immutable.
/** @type {import('@sveltejs/kit').Config} */
const config = {
  preprocess: vitePreprocess(),
  kit: {
    adapter: adapter({
      pages: "../internal/webui/dist",
      assets: "../internal/webui/dist",
      fallback: "index.html",
      strict: true,
    }),
    appDir: "assets",
  },
};

export default config;
