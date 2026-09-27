import { Link } from "../router";
import { EmptyLedgerIcon } from "../icons";

// A real, on-brand 404 instead of the previous behavior -- any unrecognized
// path silently fell through to the Projects/Your-apps list, which looked
// like the app just ignored a bad link rather than saying so.
export function NotFoundPage({ path }: { path: string }) {
  return (
    <div className="card empty-state" style={{ marginTop: 60 }}>
      <EmptyLedgerIcon />
      <p>Nothing's rooted at this address.</p>
      <div className="field-hint">
        No page matches <code className="mono">{path}</code>.
      </div>
      <div style={{ marginTop: 16 }}>
        <Link to="/" className="btn btn-primary">
          Back to solid ground
        </Link>
      </div>
    </div>
  );
}
