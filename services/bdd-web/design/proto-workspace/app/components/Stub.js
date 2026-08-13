import Link from "next/link";

// Shared shape for the new "group landing" stub pages added so every
// sidebar row (section header, sub-group, or leaf) navigates somewhere.
// Reuses the same mockup-canvas / page-header / row-list classes the
// ported pages already use (see content/epic.html, content/sessions.html)
// so these hand-written pages read as part of the same design language.
export function StubBreadcrumb({ crumbs }) {
  return (
    <nav className="mockup-breadcrumb" data-testid="mockup-breadcrumb" aria-label="Breadcrumb">
      {crumbs.map((c, i) => (
        <span key={i}>
          {i > 0 && <span className="crumb-sep">/</span>}
          {c.href ? <Link href={c.href}>{c.label}</Link> : <span aria-current="page">{c.label}</span>}
        </span>
      ))}
    </nav>
  );
}

export function StubPage({ crumbs, kicker, title, meta, children }) {
  return (
    <>
      <StubBreadcrumb crumbs={crumbs} />
      <main className="mockup-canvas canvas-max" data-testid="mockup-canvas">
        <div className="page-header">
          <div>
            <span className="section-label">{kicker}</span>
            <h1>{title}</h1>
            {meta && <p className="page-header__meta">{meta}</p>}
          </div>
        </div>
        {children}
      </main>
    </>
  );
}

export function StubRowList({ items }) {
  return (
    <div className="row-list">
      {items.map((item) => (
        <div className="list-row" key={item.href} data-testid="stub-row">
          <div className="list-row__primary">
            <div className="list-row__title">{item.title}</div>
            <div className="list-row__meta">{item.meta}</div>
          </div>
          <div className="list-row__side">
            <Link className="btn btn--solid" href={item.href}>
              Open
            </Link>
          </div>
        </div>
      ))}
    </div>
  );
}
