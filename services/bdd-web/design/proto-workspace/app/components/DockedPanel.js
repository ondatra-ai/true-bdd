"use client";

import PanelBrand from "./PanelBrand";
import { SectionTree, useDockedSectionId } from "./sections";

// The pinned/docked panel: shows ONLY the active section's tree. Lives in
// normal document flow (flex sibling of the rail), unlike the fixed/floating
// rail-hover flyout.
export default function DockedPanel() {
  const sectionId = useDockedSectionId();
  return (
    <nav className="mockup-sidebar" data-testid="mockup-sidebar" aria-label="Workspace navigation">
      <PanelBrand />
      <div className="sidebar-nav">
        <SectionTree id={sectionId} />
      </div>
    </nav>
  );
}
