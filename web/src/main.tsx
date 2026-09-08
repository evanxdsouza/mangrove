import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
// Self-hosted (no external font CDN -- this dashboard ships with the box
// it manages) faces for the instrument-panel voice: Space Grotesk for UI
// and display, IBM Plex Mono for every tabular/numeric/log readout. Latin
// + Latin Extended subsets only (the UI copy is English-only today) --
// the full @fontsource/<face>/<weight>.css also bundles Cyrillic/Greek/
// Vietnamese subsets the app never uses, which only bloats the binary
// this dashboard is embedded into for a self-hosted single-box target.
import "@fontsource/space-grotesk/latin-400.css";
import "@fontsource/space-grotesk/latin-ext-400.css";
import "@fontsource/space-grotesk/latin-500.css";
import "@fontsource/space-grotesk/latin-ext-500.css";
import "@fontsource/space-grotesk/latin-600.css";
import "@fontsource/space-grotesk/latin-ext-600.css";
import "@fontsource/space-grotesk/latin-700.css";
import "@fontsource/space-grotesk/latin-ext-700.css";
import "@fontsource/ibm-plex-mono/latin-400.css";
import "@fontsource/ibm-plex-mono/latin-ext-400.css";
import "@fontsource/ibm-plex-mono/latin-500.css";
import "@fontsource/ibm-plex-mono/latin-ext-500.css";
import "./styles.css";
import App from "./App.tsx";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
