"use client";

import { useMemo, useRef, useState } from "react";
import { usePathname, useParams } from "next/navigation";
import { useArchitectureYaml, appendRedisService, appendChatComment } from "./ArchitectureYamlContext";
import { useFiles, PRD_PATH, SCENARIOS_PATH, FEATURES_PATH } from "./FilesStore";
import {
  appendStoryStub,
  appendScenarioStub,
  deriveStories,
  parseFeaturesOutline,
  storyIdFromSlug,
  storySlugFromId,
} from "./ProductFiles";

const GREETING = {
  role: "assistant",
  text: "Hi — I'm a mocked chat, present on every page. No real model calls happen in this prototype.",
};

const DEFAULT_WIDTH = 380;
const MIN_WIDTH = 280;
const MAX_WIDTH = 640;

// Present on every workspace page (rendered once from the layout). On the
// file-backed pages (Architecture, and now Product's five files) it can
// actually mutate the shared file store; everywhere else it just replies —
// the point is the chat's presence, not a real assistant.
export default function ChatDialog() {
  const pathname = usePathname();
  const params = useParams();
  const archYaml = useArchitectureYaml();
  const filesCtx = useFiles();

  const isArchitecture = pathname === "/architecture";
  const isProduct = pathname === "/product";
  const isStory = pathname.startsWith("/story/");
  const isRequirements = pathname === "/requirements";
  const isFeatures = pathname === "/features";
  const isEditableFilePage = isArchitecture || isProduct || isStory || isRequirements || isFeatures;

  // The file the CURRENT /story/<slug> page is looking at, resolved straight
  // from the story files themselves (no epic) — needed so a generic
  // (non-"story") chat message on a story page comments on the RIGHT file.
  const currentStoryPath = useMemo(() => {
    if (!isStory) return null;
    const stories = deriveStories(filesCtx.files);
    const storyId = storyIdFromSlug(params.id || "");
    return stories.find((s) => s.id === storyId)?.file || null;
  }, [isStory, params.id, filesCtx.files]);

  const [open, setOpen] = useState(false);
  const [messages, setMessages] = useState([GREETING]);
  const [input, setInput] = useState("");
  const [width, setWidth] = useState(DEFAULT_WIDTH);
  const dragRef = useRef(null);

  // Draggable divider — optional nicety (ClickUp's Brain² panel is likely
  // resizable there too, unverified). Plain mouse listeners on window while
  // dragging; no library needed for something this small.
  function startResize(e) {
    dragRef.current = { startX: e.clientX, startWidth: width };
    function onMove(ev) {
      if (!dragRef.current) return;
      const delta = dragRef.current.startX - ev.clientX;
      const next = Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, dragRef.current.startWidth + delta));
      setWidth(next);
    }
    function onUp() {
      dragRef.current = null;
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
    }
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
  }

  function send() {
    const text = input.trim();
    if (!text) return;
    const userMsg = { role: "user", text };
    let replyText;

    if (isArchitecture && archYaml) {
      if (/redis/i.test(text)) {
        archYaml.setYaml((prev) => appendRedisService(prev));
        replyText =
          'Added a "redis" service block to architecture.yaml (image redis:7-alpine, ' +
          "port 6379, volume redisdata:/data) — see it in the editor below. You can " +
          "edit it by hand too, the file is a normal textarea.";
      } else {
        archYaml.setYaml((prev) => appendChatComment(prev, text));
        replyText =
          "Noted — I appended a comment recording that to architecture.yaml. " +
          '(This mock only scripts a couple of edits — try a message containing ' +
          '"redis" to see a real block get added.)';
      }
    } else if (isProduct && /story/i.test(text)) {
      const existingIds = deriveStories(filesCtx.files).map((s) => s.id);
      const defaultFeatureId = parseFeaturesOutline(filesCtx.files[FEATURES_PATH] || "").features[0]?.id;
      const { filePath, storyStub, newId, title } = appendStoryStub(existingIds, text, defaultFeatureId);
      filesCtx.setFile(filePath, storyStub);
      replyText =
        `Added story ${newId} — "${title}" — as a new file at ${filePath} (tagged feature: ` +
        `${defaultFeatureId || "(none)"} — change it from the story page if that's wrong). ` +
        `It already shows flat under Stories: in the sidebar; open it there ` +
        `or at /story/${storySlugFromId(newId)}.`;
    } else if (isProduct) {
      filesCtx.setFile(PRD_PATH, (prev) => appendChatComment(prev, text));
      replyText =
        "Noted — I appended a comment recording that to prd.yaml. " +
        '(This mock doesn\'t do full PRD editing yet — try a message containing ' +
        '"story" to add one, or /requirements with "scenario".)';
    } else if (isStory) {
      if (/story/i.test(text)) {
        const existingIds = deriveStories(filesCtx.files).map((s) => s.id);
        const defaultFeatureId = parseFeaturesOutline(filesCtx.files[FEATURES_PATH] || "").features[0]?.id;
        const { filePath, storyStub, newId, title } = appendStoryStub(existingIds, text, defaultFeatureId);
        filesCtx.setFile(filePath, storyStub);
        replyText =
          `Added story ${newId} — "${title}" — as a new file at ${filePath} (tagged feature: ` +
          `${defaultFeatureId || "(none)"} — change it from the story page if that's wrong). ` +
          `It already shows flat under Stories: in the sidebar; open it there ` +
          `or at /story/${storySlugFromId(newId)}.`;
      } else {
        if (currentStoryPath) {
          filesCtx.setFile(currentStoryPath, (prev) => appendChatComment(prev, text));
        }
        replyText =
          "Noted — I appended a comment to this story file. " +
          '(This mock only scripts one edit here — try a message containing "story" ' +
          "to add a brand-new one.)";
      }
    } else if (isRequirements) {
      if (/scenario/i.test(text)) {
        const scenariosYaml = filesCtx.files[SCENARIOS_PATH] || "";
        const { updatedScenarios, newId } = appendScenarioStub(scenariosYaml, text);
        filesCtx.setFile(SCENARIOS_PATH, updatedScenarios);
        replyText = `Added scenario ${newId} to scenarios.yaml — see it in the editor and under Scenarios: in the sidebar.`;
      } else {
        filesCtx.setFile(SCENARIOS_PATH, (prev) => appendChatComment(prev, text));
        replyText =
          "Noted — I appended a comment to scenarios.yaml. " +
          '(Try a message containing "scenario" to add a new one.)';
      }
    } else {
      replyText = `Got it — I'll treat "${text}" as a note on this page. (Mocked reply, no live model call.)`;
    }

    setMessages((m) => [...m, userMsg, { role: "assistant", text: replyText }]);
    setInput("");
  }

  return (
    <div className={`chat-dock${open ? " is-open" : ""}`} data-testid="chat-dock">
      {open && (
        <div
          className="chat-dock__resizer"
          onMouseDown={startResize}
          data-testid="chat-dock-resizer"
          role="separator"
          aria-orientation="vertical"
          aria-label="Resize chat panel"
        />
      )}
      {open && (
        <div className="chat-dock__panel" style={{ width }} data-testid="chat-dock-panel">
          <div className="chat-dock__header">
            <span className="section-label">Chat</span>
            {isEditableFilePage && <span className="chip">editing this file</span>}
          </div>

          <div className="chat-dock__history">
            {messages.map((m, i) => (
              <div className={`chat-msg chat-msg--${m.role}`} key={i}>
                <span className="chat-msg__role">{m.role === "user" ? "You" : "Assistant"}</span>
                <p className="chat-msg__text">{m.text}</p>
              </div>
            ))}
          </div>

          <div className="chat-dock__input-row">
            <input
              type="text"
              placeholder={isEditableFilePage ? 'Try an edit request…' : "Message…"}
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") send();
              }}
              aria-label="Chat message"
            />
            <button
              type="button"
              className="btn btn--solid"
              onClick={send}
              disabled={!input.trim()}
            >
              Send
            </button>
          </div>
        </div>
      )}

      <button
        type="button"
        className="chat-dock__tab"
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        data-testid="chat-dock-toggle"
      >
        <span className="chat-dock__tab-label">{open ? "Close chat" : "Chat"}</span>
      </button>
    </div>
  );
}
