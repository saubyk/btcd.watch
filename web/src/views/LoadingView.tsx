export function LoadingView() {
  return (
    <main className="bp-view bp-result">
      <div className="bp-loading-note">
        <span className="bp-spinner" />
        Searching the blockchain…
      </div>
      <div className="bp-skeleton-card">
        <div className="bp-skeleton-body">
          <div className="bp-skel bp-skel--title bp-shimmer" />
          <div className="bp-skel bp-skel--amount bp-shimmer" />
          <div className="bp-skel-tiles">
            <div className="bp-skel bp-skel--tile bp-shimmer" />
            <div className="bp-skel bp-skel--tile bp-shimmer" />
            <div className="bp-skel bp-skel--tile bp-shimmer" />
          </div>
        </div>
      </div>
    </main>
  )
}
