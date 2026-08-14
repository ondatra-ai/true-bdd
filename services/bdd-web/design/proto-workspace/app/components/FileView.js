"use client";

import { useEffect, useRef, useState } from "react";
import { useFiles } from "./FilesStore";

// Must match .yaml-editor__gutter / .yaml-editor__textarea line-height in
// public/proto-extra.css.
const LINE_HEIGHT = 20;
const GUTTER_TOP_PADDING = 10;
const FLASH_MS = 900;

// GitHub-file-view-style whole-file rendering, generalized out of
// /architecture's original page so every Product file page (product.yaml, the
// epic file, each story file, scenarios.yaml) can reuse it instead of
// duplicating the header-bar/gutter/textarea/anchor/flash markup.
// /architecture itself is left on its own inline implementation
// (unchanged, already validated) to avoid touching working code.
export default function FileView({ kicker, title, meta, path, content, onChange, anchors, beforeFile }) {
  const { jumpRequest } = useFiles();
  const lines = content.split("\n");
  const [flashLine, setFlashLine] = useState(null);

  // Autosave state chip (report F13.2). In-memory prototype, so mimic prod's
  // idle → saving → saved cycle off each edit; skip the initial mount so an
  // untouched file reads "idle" rather than flashing "saved" on load.
  const [saveState, setSaveState] = useState("idle");
  const didMountRef = useRef(false);
  useEffect(() => {
    if (!didMountRef.current) {
      didMountRef.current = true;
      return;
    }
    setSaveState("saving");
    const t = setTimeout(() => setSaveState("saved"), 700);
    return () => clearTimeout(t);
  }, [content]);

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
    <main className="mockup-canvas canvas-max" data-testid="mockup-canvas">
      <div className="page-header">
        <div>
          <span className="section-label">{kicker}</span>
          <h1>{title}</h1>
          {meta && <p className="page-header__meta">{meta}</p>}
        </div>
      </div>

      {beforeFile}

      <div className="file-view">
        <div className="file-view__bar">
          <span className="file-view__path">{path}</span>
          <span className="file-view__meta">
            <span
              className={`file-view__status file-view__status--${saveState}`}
              data-testid="file-view-status"
              aria-live="polite"
            >
              {saveState}
            </span>
            <span className="file-view__lines">{lines.length} lines</span>
          </span>
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
            value={content}
            onChange={(e) => onChange(e.target.value)}
            spellCheck={false}
            wrap="off"
            rows={lines.length}
            aria-label={`${path} contents — editable`}
            data-testid="yaml-editor-textarea"
          />
        </div>
      </div>
    </main>
  );
}
