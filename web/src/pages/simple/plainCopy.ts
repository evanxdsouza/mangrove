export interface PlainTemplateCopy {
  label: string;
  blurb: string;
}

// Simple mode only offers user-facing apps -- standalone databases
// (postgres/mysql/mongodb/redis) exist as backing services other
// templates install alongside themselves, not something a non-technical
// user would knowingly add on their own. Any template key missing here
// (a future addition) is simply left out of the simple-mode gallery
// rather than showing its technical name/description unfiltered.
export const SIMPLE_TEMPLATES: Record<string, PlainTemplateCopy> = {
  ghost: { label: "Blog", blurb: "Write and publish posts on your own site." },
  wordpress: { label: "Website builder", blurb: "Build a flexible website or blog." },
  vaultwarden: { label: "Password manager", blurb: "Store and share passwords securely, on your own site." },
  "uptime-kuma": { label: "Website monitor", blurb: "Get notified if one of your sites goes down." },
  n8n: { label: "Automation", blurb: "Connect your apps together and automate busywork." },
  umami: { label: "Visitor stats", blurb: "See how many people visit your sites, without tracking them." },
  nocodb: { label: "Spreadsheet database", blurb: "Turn a spreadsheet into a shareable database, no code needed." },
  pocketbase: { label: "App backend", blurb: "Ship logins, data, and APIs from one lightweight backend." },
};

export function plainStatus(status: string): string {
  switch (status) {
    case "running":
      return "Running";
    case "pending":
    case "building":
      return "Starting up";
    case "stopped":
      return "Turned off";
    case "failed":
      return "Having a problem";
    default:
      return status;
  }
}
