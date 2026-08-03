"use client";

import { useRef, useState } from "react";
import Link from "next/link";
import PanelBrand from "./PanelBrand";
import { SECTIONS, SectionTree, useActiveSection } from "./sections";

const OPEN_DELAY_MS = 150;
const CLOSE_DELAY_MS = 150;

// The narrow far-left rail (ClickUp's Home/Spaces/Chat/Docs bar, adapted to
// the mockup's square/invert B&W language instead of rounded/purple).
// Hovering a non-active item opens a flyout of that section's tree after a
// short delay; clicking navigates (which — because the active section is
// derived from the route — also docks that section, closing the flyout).
export default function Rail() {
  const activeId = useActiveSection();
  const [hoveredId, setHoveredId] = useState(null);
  const openTimer = useRef(null);
  const closeTimer = useRef(null);

  function clearTimers() {
    clearTimeout(openTimer.current);
    clearTimeout(closeTimer.current);
  }

  function scheduleOpen(id) {
    clearTimers();
    openTimer.current = setTimeout(() => setHoveredId(id), OPEN_DELAY_MS);
  }

  function scheduleClose() {
    clearTimers();
    closeTimer.current = setTimeout(() => setHoveredId(null), CLOSE_DELAY_MS);
  }

  function cancelClose() {
    clearTimeout(closeTimer.current);
  }

  // Never flyout the section that's already docked — hovering the active
  // rail item just re-affirms it, nothing floats over the page.
  const flyoutId = hoveredId && hoveredId !== activeId ? hoveredId : null;

  return (
    <div className="workspace-rail-wrap">
      <nav className="workspace-rail" data-testid="workspace-rail" aria-label="Sections">
        {SECTIONS.map((s) => (
          <Link
            key={s.id}
            href={s.href}
            data-testid={`rail-item-${s.id}`}
            className={`rail-item${s.id === activeId ? " is-active" : ""}`}
            onMouseEnter={() => scheduleOpen(s.id)}
            onMouseLeave={scheduleClose}
          >
            <span className="rail-item__icon" aria-hidden="true">
              {s.icon}
            </span>
            <span className="rail-item__label">{s.label}</span>
          </Link>
        ))}

        <Link href="/sessions" className="rail-item rail-item--utility">
          <span className="rail-item__icon" aria-hidden="true">
            ↩
          </span>
          <span className="rail-item__label">Sessions</span>
        </Link>
      </nav>

      {flyoutId && (
        <div
          className="mockup-sidebar rail-flyout"
          data-testid="rail-flyout"
          onMouseEnter={cancelClose}
          onMouseLeave={scheduleClose}
        >
          <PanelBrand />
          <div className="sidebar-nav">
            <SectionTree id={flyoutId} />
          </div>
        </div>
      )}
    </div>
  );
}
