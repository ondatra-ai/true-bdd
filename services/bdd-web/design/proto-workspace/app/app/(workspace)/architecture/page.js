"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import {
  useArchitectureYaml,
  parseArchitectureOutline,
  outlineAnchors,
} from "../../../components/ArchitectureYamlContext";

// Must match .yaml-editor__gutter / .yaml-editor__textarea line-height in
// public/proto-extra.css — used to place the invisible anchor markers the
// sidebar (and this page's own jump effect) scroll to.
const LINE_HEIGHT = 20;
const GUTTER_TOP_PADDING = 10;
const FLASH_MS = 900;

export default function ArchitecturePage() {
  const { yaml, setYaml, jumpRequest } = useArchitectureYaml();
  const lines = yaml.split("\n");
  const [flashLine, setFlashLine] = useState(null);

  // Recomputed on every edit (chat OR direct typing) so the sidebar outline
  // and this page's own scroll targets never go stale.
  const outline = useMemo(() => parseArchitectureOutline(yaml), [yaml]);
  const anchors = useMemo(() => outlineAnchors(outline), [outline]);

  // A sidebar/rail row called requestJump(id) — scroll .app-main__content
  // (the actual scroller; .app-frame is viewport-capped) so that line lands
  // near the top, then briefly flash it. Runs on mount too, so navigating
  // in from a totally different page still lands + jumps correctly.
  useEffect(() => {
    if (!jumpRequest) return;
    const el = document.getElementById(jumpRequest.id);
    if (!el) return;
    el.scrollIntoView({ behavior: "smooth", block: "start" });
    const anchor = anchors.find((a) => a.id === jumpRequest.id);
    if (anchor) {
      setFlashLine(anchor.line);
      const t = setTimeout(() => setFlashLine(null), FLASH_MS);
      return () => clearTimeout(t);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [jumpRequest]);

  return (
    <>
      <nav
        className="mockup-breadcrumb"
        data-testid="mockup-breadcrumb"
        aria-label="Breadcrumb"
      >
        <Link href="/sessions">Sessions</Link>
        <span className="crumb-sep">/</span>
        <Link href="/workspace-overview">Workspace overview</Link>
        <span className="crumb-sep">/</span>
        <span aria-current="page">architecture.yaml</span>
      </nav>

      <main className="mockup-canvas canvas-max" data-testid="mockup-canvas">
        <div className="page-header">
          <div>
            <span className="section-label">01—Architecture</span>
            <h1>architecture.yaml</h1>
            <p className="page-header__meta">
              Whole-file view, GitHub-style. Edit directly below, or ask the chat
              (bottom right) to change it for you — either way the sidebar
              outline on the left updates live.
            </p>
          </div>
        </div>

        <div className="file-view">
          <div className="file-view__bar">
            <span className="file-view__path">docs/architecture/architecture.yaml</span>
            <span className="file-view__meta">{lines.length} lines</span>
          </div>
          <div className="yaml-editor" data-testid="yaml-editor">
            {anchors.map(({ id, line }) => (
              <span
                key={id}
                id={id}
                className="yaml-editor__anchor"
                style={{ top: line * LINE_HEIGHT + GUTTER_TOP_PADDING }}
              />
            ))}
            {flashLine !== null && (
              <div
                className="yaml-editor__flash"
                data-testid="yaml-editor-flash"
                style={{ top: flashLine * LINE_HEIGHT + GUTTER_TOP_PADDING }}
              />
            )}
            <pre className="yaml-editor__gutter" aria-hidden="true">
              {lines.map((_, i) => i + 1).join("\n")}
            </pre>
            <textarea
              className="yaml-editor__textarea"
              value={yaml}
              onChange={(e) => setYaml(e.target.value)}
              spellCheck={false}
              wrap="off"
              rows={lines.length}
              aria-label="architecture.yaml contents — editable"
              data-testid="yaml-editor-textarea"
            />
          </div>
        </div>
      </main>
    </>
  );
}
