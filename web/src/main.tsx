import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
// Self-hosted (no external font CDN -- this dashboard ships with the box
// it manages) faces for the Intertidal voice: Figtree for UI and display,
// JetBrains Mono for slugs, ports, env, and logs. Latin + Latin Extended
// subsets only (the UI copy is English-only today) -- the full
// @fontsource/<face>/<weight>.css also bundles Cyrillic/Greek/Vietnamese
// subsets the app never uses, which only bloats the binary this dashboard
// is embedded into for a self-hosted single-box target.
import "@fontsource/figtree/latin-400.css";
import "@fontsource/figtree/latin-ext-400.css";
import "@fontsource/figtree/latin-500.css";
import "@fontsource/figtree/latin-ext-500.css";
import "@fontsource/figtree/latin-600.css";
import "@fontsource/figtree/latin-ext-600.css";
import "@fontsource/figtree/latin-700.css";
import "@fontsource/figtree/latin-ext-700.css";
import "@fontsource/jetbrains-mono/latin-400.css";
import "@fontsource/jetbrains-mono/latin-ext-400.css";
import "@fontsource/jetbrains-mono/latin-500.css";
import "@fontsource/jetbrains-mono/latin-ext-500.css";
import "./styles.css";
import App from "./App.tsx";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
