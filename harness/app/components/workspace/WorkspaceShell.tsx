"use client";

/**
 * The persistent workspace app-shell (P1–P3, P11/P12): icon rail (dock +
 * active + hover flyout), docked sidebar, the scrolling content pane, and the
 * docked chat. Everything here lives INSIDE `FilesProvider` and stays
 * mounted across client-side navigation within `(workspace)` (Established
 * fact) — the rail/sidebar/chat state and every open document's buffer
 * survive route changes.
 */

import { useEffect, useRef, useState } from "react";
import type { PointerEvent, ReactNode } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";

import { effectiveContent, FilesProvider, useFiles } from "@/app/lib/workspace/files-context";
import { routes } from "@/app/lib/workspace/routes";
import type { WorkspaceSection } from "@/app/lib/workspace/types";

import { SectionTree } from "./SectionTree";

const SECTIONS: WorkspaceSection[] = ["home", "architecture", "product", "builds"];

const SECTION_LABEL: Record<WorkspaceSection, string> = {
  home: "Home",
  architecture: "Arch",
  product: "Prod",
  builds: "Bld",
};

function sectionHref(sid: string, section: WorkspaceSection): string {
  switch (section) {
    case "home":
      return routes.home(sid);
    case "architecture":
      return routes.architecture(sid);
    case "product":
      return routes.product(sid);
    case "builds":
      return routes.builds(sid);
  }
}

function sectionFromPathname(pathname: string, sid: string): WorkspaceSection {
  if (pathname.startsWith(`/sessions/${sid}/architecture`)) return "architecture";
  if (pathname.startsWith(`/sessions/${sid}/product`)) return "product";
  if (pathname.startsWith(`/sessions/${sid}/builds`)) return "builds";

  return "home";
}

function Rail({ currentSection }: { currentSection: WorkspaceSection }) {
  const { sid, openFlyout, scheduleCloseFlyout } = useFiles();

  return (
    <nav data-testid="rail" data-active-section={currentSection}>
      <div className="ws-rail-items">
        {SECTIONS.map((section) => (
          <Link
            key={section}
            data-testid={`rail-item-${section}`}
            data-section={section}
            aria-current={section === currentSection ? "page" : undefined}
            href={sectionHref(sid, section)}
            className="ws-rail-item"
            onMouseEnter={() => {
              if (section !== currentSection) {
                openFlyout(section);
              }
            }}
            onMouseLeave={scheduleCloseFlyout}
          >
            {SECTION_LABEL[section]}
          </Link>
        ))}
      </div>
      <div data-testid="rail-utilities">
        <button type="button" data-testid="rail-utility-item" className="ws-rail-utility" aria-label="Settings">
          {"Set"}
        </button>
      </div>
    </nav>
  );
}

function RailFlyout({ currentSection }: { currentSection: WorkspaceSection }) {
  const { hoveredSection, openFlyout, scheduleCloseFlyout } = useFiles();

  if (hoveredSection === null || hoveredSection === currentSection) {
    return null;
  }

  return (
    <div
      data-testid="rail-flyout"
      onMouseEnter={() => openFlyout(hoveredSection)}
      onMouseLeave={scheduleCloseFlyout}
    >
      <SectionTree section={hoveredSection} />
    </div>
  );
}

function Sidebar({ currentSection }: { currentSection: WorkspaceSection }) {
  return (
    <aside data-testid="sidebar">
      <div data-testid={`sidebar-section-${currentSection}`}>
        <SectionTree section={currentSection} />
      </div>
    </aside>
  );
}

function ChatDock() {
  const { chatOpen, setChatOpen, chatWidth, setChatWidth, conversation, sendChatMessage } = useFiles();
  const [text, setText] = useState("");
  const [sending, setSending] = useState(false);
  const dragState = useRef<{ startX: number; startWidth: number } | null>(null);

  function onResizerPointerDown(event: PointerEvent<HTMLDivElement>) {
    dragState.current = { startX: event.clientX, startWidth: chatWidth };

    function onMove(moveEvent: globalThis.PointerEvent) {
      if (dragState.current === null) return;
      // The resizer sits at the panel's LEFT edge; dragging it LEFT (away
      // from the content pane) widens the panel.
      const delta = dragState.current.startX - moveEvent.clientX;
      setChatWidth(Math.max(240, dragState.current.startWidth + delta));
    }

    function onUp() {
      dragState.current = null;
      window.removeEventListener("pointermove", onMove);
      window.removeEventListener("pointerup", onUp);
    }

    window.addEventListener("pointermove", onMove);
    window.addEventListener("pointerup", onUp);
  }

  async function handleSend() {
    const value = text.trim();
    if (value === "" || sending) return;
    setText("");
    setSending(true);
    try {
      await sendChatMessage(value);
    } finally {
      setSending(false);
    }
  }

  if (!chatOpen) {
    return (
      <button
        type="button"
        data-testid="chat-dock-toggle"
        aria-label="Open chat"
        onClick={() => setChatOpen(true)}
      >
        {"‹"}
      </button>
    );
  }

  return (
    <div data-testid="chat-dock">
      <div data-testid="chat-dock-resizer" onPointerDown={onResizerPointerDown} />
      <div data-testid="chat-dock-panel" style={{ width: chatWidth }}>
        <div data-testid="chat-dock-header">
          <span>Chat</span>
          <button
            type="button"
            data-testid="chat-dock-new"
            onClick={() => {
              /* Starting a fresh conversation is not required by any spec;
               * this control exists to satisfy the ClickUp parity header. */
            }}
          >
            + New
          </button>
          <button type="button" data-testid="chat-dock-toggle" aria-label="Close chat" onClick={() => setChatOpen(false)}>
            {"›"}
          </button>
        </div>
        <div data-testid="chat-dock-history">
          {conversation.map((message, index) => (
            <div key={index} data-testid="chat-dock-message" data-role={message.role} className="ws-chat-message">
              {message.content}
            </div>
          ))}
        </div>
        <div className="ws-chat-input-row">
          <input
            data-testid="chat-dock-input"
            value={text}
            onChange={(event) => setText(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                void handleSend();
              }
            }}
          />
          <button type="button" data-testid="chat-dock-send" onClick={() => void handleSend()}>
            Send
          </button>
        </div>
      </div>
    </div>
  );
}

function ShellBody({ sid, children }: { sid: string; children: ReactNode }) {
  const { loading, navigating, endNavigation } = useFiles();
  const pathname = usePathname();
  const currentSection = sectionFromPathname(pathname, sid);

  // The navigating flag is cleared once the DESTINATION page has actually
  // committed (this effect re-runs whenever `pathname` changes, which only
  // happens once the new route's tree is the one rendered) — see the
  // `navigating` doc-comment in files-context.tsx for why this matters.
  useEffect(() => {
    endNavigation();
  }, [pathname, endNavigation]);

  // Gate the WHOLE shell (not just content-pane) on the initial doc_tree +
  // doc_read load (plan Slice 0): NOTHING in the workspace is interactive —
  // not the rail, not the chat toggle, not the content — before every
  // document's buffer/revision is populated. Without this, a fast script can
  // open the chat and send a message before FileView has registered itself
  // as the "current document" (current_path would read null — P10's chat
  // target binding — or a form submit could race an unset base_revision).
  // Every caller already waits on a testid appearing (`.click()` auto-waits
  // for the element to exist), so rendering nothing meanwhile is safe.
  if (loading) {
    return <div data-testid="app-shell" />;
  }

  return (
    <div data-testid="app-shell">
      <Rail currentSection={currentSection} />
      <RailFlyout currentSection={currentSection} />
      <Sidebar currentSection={currentSection} />
      {/* Also gate on an in-flight cross-page jump: never show the PREVIOUS
       * page's file view once a jump has been requested, only the
       * destination's (see the `navigating` doc-comment in
       * files-context.tsx). */}
      <div data-testid="content-pane">{navigating ? null : children}</div>
      <ChatDock />
    </div>
  );
}

export function WorkspaceShell({ sid, children }: { sid: string; children: ReactNode }) {
  return (
    <FilesProvider sid={sid}>
      <ShellBody sid={sid}>{children}</ShellBody>
    </FilesProvider>
  );
}

// Re-exported for pages that need to read the live buffer directly.
export { effectiveContent };
