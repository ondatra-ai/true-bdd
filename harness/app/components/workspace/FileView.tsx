"use client";

/**
 * GitHub-style file view (P8/P17): a path header, a line-number gutter, and
 * an edit-in-place textarea. Registers itself as the workspace's "current
 * file" for chat targeting (P10) and consumes a pending cross-page jump
 * (P14) — navigate first (the caller does that), then this component scrolls
 * once it is the mounted page for `docPath`.
 */

import { useEffect, useRef, useState } from "react";

import { effectiveContent, useFiles } from "@/app/lib/workspace/files-context";
import { isValidYAML } from "@/app/lib/workspace/yaml-utils";

const LINE_HEIGHT_PX = 20;

export function FileView({ docPath }: { docPath: string }) {
  const { getDoc, setBuffer, registerCurrentDoc, pendingJump, clearJump } = useFiles();
  const doc = getDoc(docPath);
  const editorRef = useRef<HTMLTextAreaElement | null>(null);
  const gutterRef = useRef<HTMLDivElement | null>(null);
  const [flashLine, setFlashLine] = useState<number | null>(null);

  useEffect(() => {
    registerCurrentDoc(docPath);

    return () => registerCurrentDoc(null);
  }, [docPath, registerCurrentDoc]);

  useEffect(() => {
    if (pendingJump !== null && pendingJump.path === docPath) {
      const editor = editorRef.current;
      if (editor !== null) {
        editor.scrollTop = pendingJump.line * LINE_HEIGHT_PX;
        if (gutterRef.current !== null) {
          gutterRef.current.scrollTop = editor.scrollTop;
        }
      }
      setFlashLine(pendingJump.line);
      clearJump();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pendingJump, docPath]);

  if (doc === undefined) {
    return null;
  }

  const buffer = doc.buffer;
  const valid = isValidYAML(buffer);
  const lineCount = buffer.length === 0 ? 1 : buffer.split("\n").length;

  return (
    <div data-testid="file-view" className="ws-file-view">
      <div data-testid="file-view-path" className="ws-file-view-path">
        {docPath}
      </div>
      <div className="ws-file-view-body">
        <div data-testid="file-view-gutter" ref={gutterRef} className="ws-file-view-gutter">
          {Array.from({ length: lineCount }, (_, i) => (
            <div key={i} data-testid="file-view-gutter-line" className="ws-gutter-line">
              {i + 1}
            </div>
          ))}
        </div>
        <textarea
          data-testid="file-view-editor"
          className="ws-file-view-editor"
          spellCheck={false}
          value={buffer}
          onChange={(event) => setBuffer(docPath, event.target.value)}
          onScroll={(event) => {
            if (gutterRef.current !== null) {
              gutterRef.current.scrollTop = event.currentTarget.scrollTop;
            }
          }}
          ref={editorRef}
        />
        {flashLine !== null && (
          <div
            data-testid="file-view-flash"
            data-line={flashLine}
            className="ws-file-view-flash"
            style={{ top: flashLine * LINE_HEIGHT_PX }}
          />
        )}
      </div>
      {!valid && (
        <div data-testid="yaml-invalid-indicator" className="ws-invalid-indicator">
          Invalid YAML — showing the last valid outline until this parses again.
        </div>
      )}
      <div data-testid="save-state" data-save-state={doc.saveState} data-revision={doc.syncedRevision} className="ws-save-state">
        {doc.saveState}
      </div>
    </div>
  );
}

/** Convenience re-export so pages deriving outlines share ONE definition of
 * "which content to derive from" (buffer when valid, else last-valid). */
export { effectiveContent };
