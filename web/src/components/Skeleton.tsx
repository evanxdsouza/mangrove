// Shimmering placeholder shapes shown while a page's first fetch is still
// in flight -- used in place of CenterLoading on grids/lists specifically,
// so the page's real layout is already visible (card outlines, roughly
// right text widths) instead of a spinner replacing the whole content
// area. The shimmer is a plain CSS animation, so it's automatically
// stopped by the global prefers-reduced-motion rule in styles.css.

export function SkeletonLine({ width = "100%", height = 12 }: { width?: string | number; height?: number }) {
  return <span className="skeleton-line" style={{ width, height }} />;
}

export function SkeletonCircle({ size = 20 }: { size?: number }) {
  return <span className="skeleton-circle" style={{ width: size, height: size }} />;
}

/** Matches .project-card's shape: icon + name/slug, description line, footer. */
export function ProjectCardSkeleton() {
  return (
    <div className="card skeleton-card">
      <div className="project-card-top">
        <SkeletonCircle size={20} />
        <div className="project-card-heading" style={{ gap: 6 }}>
          <SkeletonLine width="60%" height={14} />
          <SkeletonLine width="35%" height={10} />
        </div>
      </div>
      <SkeletonLine width="80%" height={12} />
      <div className="project-card-footer" style={{ marginTop: 10 }}>
        <SkeletonLine width="50%" height={11} />
      </div>
    </div>
  );
}

/** Matches .project-list-row's shape -- a single full-width line. */
export function ProjectRowSkeleton() {
  return (
    <div className="card skeleton-card project-list-row">
      <div className="project-list-row-main">
        <span className="project-list-row-chevron-spacer" />
        <SkeletonCircle size={20} />
        <div className="project-list-row-heading" style={{ gap: 6 }}>
          <SkeletonLine width="30%" height={14} />
          <SkeletonLine width="20%" height={10} />
        </div>
        <SkeletonLine width={80} height={20} />
      </div>
    </div>
  );
}

/** Matches .deployment-card's shape. */
export function DeploymentCardSkeleton() {
  return (
    <div className="card skeleton-card">
      <div className="deployment-card-top">
        <SkeletonCircle size={20} />
        <SkeletonLine width="50%" height={14} />
      </div>
      <SkeletonLine width="40%" height={11} />
      <div style={{ marginTop: 10 }}>
        <SkeletonLine width="60%" height={11} />
      </div>
    </div>
  );
}
