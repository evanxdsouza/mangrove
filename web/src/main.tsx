import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
// Self-hosted (no external font CDN) faces: Inter for all UI text, JetBrains
// Mono only for code, logs, IDs and terminal output. Latin subsets only.
import "@fontsource-variable/inter/wght.css";
import "@fontsource/jetbrains-mono/latin-400.css";
import "@fontsource/jetbrains-mono/latin-500.css";
import "./styles.css";
import App from "./App.tsx";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
