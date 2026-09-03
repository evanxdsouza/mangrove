import { Component, type ReactNode } from "react";

interface Props {
  children: ReactNode;
}

interface State {
  error: Error | null;
}

// A crash while rendering one page (a bad API response shape, a null
// dereference) previously took down the entire SPA -- there was no
// boundary anywhere, so React unmounts from the root, and the whole
// dashboard (including the sidebar nav) goes blank until a hard reload.
// This scopes that failure to the page body: the nav shell around it
// (Layout, in App.tsx) keeps working, and switching pages recovers
// automatically since App.tsx keys this by path, remounting it fresh.
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: { componentStack?: string | null }) {
    console.error("Unhandled error rendering page:", error, info.componentStack);
  }

  render() {
    if (this.state.error) {
      return (
        <div className="card empty-state">
          <p>Something went wrong loading this page.</p>
          <p className="text-dim mono" style={{ fontSize: 12 }}>
            {this.state.error.message}
          </p>
        </div>
      );
    }
    return this.props.children;
  }
}
