"use client";

/**
 * FilesProvider — the session-scoped persistent store behind the whole
 * workspace (plan "Derivation model"). Loads the manifest + every file's
 * bytes from the CLI (doc_tree/doc_read), holds the live editing buffer per
 * document, debounces autosave through doc_write with base_revision, keeps
 * the last-valid content on a parse error (P10b), and exposes the docked
 * chat + sidebar-expand + hover-flyout + cross-page-jump state — everything
 * that must SURVIVE client-side navigation lives here, in the persistent
 * `(workspace)` layout (Established fact: a persistent route-group layout
 * keeps state across nav).
 */

import { createContext, useCallback, useContext, useEffect, useReducer, useRef, useState } from "react";
import type { ReactNode } from "react";

import { fetchDocRead, fetchDocTree, postChat, postDocWrite } from "./api";
import type { ChatMessage, DocState, DocTreeEntry, PendingJump, SaveState, WorkspaceSection } from "./types";
import { isValidYAML } from "./yaml-utils";

const AUTOSAVE_DEBOUNCE_MS = 400;
const FLYOUT_CLOSE_DELAY_MS = 150;

interface FilesContextValue {
  sid: string;
  manifest: DocTreeEntry[];
  loading: boolean;
  getDoc: (path: string) => DocState | undefined;
  setBuffer: (path: string, value: string) => void;
  /** Bypasses the debounce and writes NOW — for multi-step flows (P22). */
  commitBufferNow: (path: string, content: string, baseRevisionOverride?: string) => Promise<boolean>;
  refetchTree: () => Promise<void>;

  registerCurrentDoc: (path: string | null) => void;
  currentDocPath: string | null;

  pendingJump: PendingJump | null;
  requestJump: (path: string, line: number) => void;
  clearJump: () => void;

  chatOpen: boolean;
  setChatOpen: (open: boolean) => void;
  chatWidth: number;
  setChatWidth: (w: number) => void;
  conversation: ChatMessage[];
  sendChatMessage: (text: string) => Promise<void>;

  isGroupExpanded: (key: string) => boolean;
  toggleGroup: (key: string) => void;

  hoveredSection: WorkspaceSection | null;
  openFlyout: (section: WorkspaceSection) => void;
  scheduleCloseFlyout: () => void;
  closeFlyoutNow: () => void;

  /**
   * True from the INSTANT a cross-page outline jump starts until the target
   * page has actually mounted. Next.js App Router client-side navigations
   * are wrapped in a React transition, which by design keeps the PREVIOUS
   * page's DOM (incl. its `file-view-editor`) on screen until the new page
   * is ready — a real hazard for a test that reads/writes the editor
   * immediately after a navigating click with no intervening wait (it could
   * observe the PREVIOUS file, then commit that stale content to the NEW
   * path once the swap lands mid-flight — a torn read across the
   * navigation). Setting this flag is a PLAIN (non-transition) state update,
   * so it commits synchronously within the same click handler, making the
   * content pane render nothing until the destination is mounted — turning
   * the race into a deterministic wait (Playwright's `locator.evaluate()`
   * already waits for a matching element to exist).
   */
  navigating: boolean;
  beginNavigation: () => void;
  endNavigation: () => void;
}

const FilesContext = createContext<FilesContextValue | null>(null);

function randomToken(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }

  return `tok-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

export function FilesProvider({ sid, children }: { sid: string; children: ReactNode }) {
  const docsRef = useRef<Record<string, DocState>>({});
  const [, forceRender] = useReducer((c: number) => c + 1, 0);
  const [manifest, setManifest] = useState<DocTreeEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const timers = useRef<Record<string, ReturnType<typeof setTimeout>>>({});

  const currentDocPathRef = useRef<string | null>(null);
  const [currentDocPath, setCurrentDocPathState] = useState<string | null>(null);

  const [pendingJump, setPendingJump] = useState<PendingJump | null>(null);

  const [chatOpen, setChatOpen] = useState(false);
  const [chatWidth, setChatWidth] = useState(768);
  const conversationRef = useRef<ChatMessage[]>([]);
  const [conversation, setConversationState] = useState<ChatMessage[]>([]);

  const [groupExpanded, setGroupExpanded] = useState<Record<string, boolean>>({});

  const [hoveredSection, setHoveredSection] = useState<WorkspaceSection | null>(null);
  const flyoutTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const [navigating, setNavigating] = useState(false);
  const beginNavigation = useCallback(() => setNavigating(true), []);
  const endNavigation = useCallback(() => setNavigating(false), []);

  const updateDoc = useCallback((path: string, patch: Partial<DocState>) => {
    docsRef.current = { ...docsRef.current, [path]: { ...docsRef.current[path], ...(patch as DocState) } };
    forceRender();
  }, []);

  const loadDoc = useCallback(
    async (path: string) => {
      const result = await fetchDocRead(sid, path);
      if (result === null) {
        return;
      }
      updateDoc(path, {
        buffer: result.content,
        lastValidContent: result.parse_status === "ok" ? result.content : "",
        syncedRevision: result.revision,
        saveState: "idle",
        exists: result.exists,
      });
    },
    [sid, updateDoc],
  );

  const refetchTree = useCallback(async () => {
    const tree = await fetchDocTree(sid);
    setManifest(tree);
    const missing = tree.filter((entry) => docsRef.current[entry.path] === undefined);
    await Promise.all(missing.map((entry) => loadDoc(entry.path)));
  }, [sid, loadDoc]);

  useEffect(() => {
    let cancelled = false;

    async function boot() {
      setLoading(true);
      const tree = await fetchDocTree(sid);
      if (cancelled) return;
      setManifest(tree);
      await Promise.all(tree.map((entry) => loadDoc(entry.path)));
      if (!cancelled) setLoading(false);
    }

    void boot();

    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sid]);

  /**
   * Writes `content` for `path` NOW (bypassing the debounce timer — and
   * cancelling any pending one), updating doc state from the result.
   * Returns whether the write was accepted, so a multi-step flow (P22's
   * feature-stub-first-then-story-file ordering) can await each step.
   */
  const commitBufferNow = useCallback(
    async (path: string, content: string, baseRevisionOverride?: string): Promise<boolean> => {
      const existingTimer = timers.current[path];
      if (existingTimer !== undefined) {
        clearTimeout(existingTimer);
        delete timers.current[path];
      }

      const doc = docsRef.current[path];
      const baseRevision = baseRevisionOverride ?? doc?.syncedRevision ?? "absent";

      updateDoc(path, { buffer: content, saveState: "saving" as SaveState });
      const result = await postDocWrite(sid, path, content, baseRevision, randomToken());

      if (result.status === 200 && result.receipt !== undefined) {
        updateDoc(path, {
          saveState: "saved" as SaveState,
          lastValidContent: content,
          syncedRevision: result.receipt.committed_revision,
          exists: true,
        });

        return true;
      }

      const failState: SaveState =
        result.status === 409 ? "conflict" : result.status === 422 ? "invalid" : "error";
      updateDoc(path, { saveState: failState });

      return false;
    },
    [sid, updateDoc],
  );

  const commitSave = useCallback(
    async (path: string) => {
      const doc = docsRef.current[path];
      if (doc === undefined) return;

      // Re-validate at fire time so a debounced autosave NEVER commits a buffer
      // that no longer parses (P10a: "not committed until it parses"). The
      // scheduler already cancels a pending timer when an edit turns invalid, so
      // this is a defensive belt independent of that timer lifecycle — a buffer
      // that went invalid after the timer was armed is surfaced, not written.
      if (!isValidYAML(doc.buffer)) {
        updateDoc(path, { saveState: "invalid" as SaveState });

        return;
      }

      await commitBufferNow(path, doc.buffer, doc.syncedRevision);
    },
    [commitBufferNow, updateDoc],
  );

  const scheduleSave = useCallback(
    (path: string, value: string) => {
      const existing = timers.current[path];
      if (existing !== undefined) {
        clearTimeout(existing);
      }

      if (!isValidYAML(value)) {
        updateDoc(path, { saveState: "invalid" as SaveState });

        return;
      }

      updateDoc(path, { saveState: "saving" as SaveState });
      timers.current[path] = setTimeout(() => {
        void commitSave(path);
      }, AUTOSAVE_DEBOUNCE_MS);
    },
    [commitSave, updateDoc],
  );

  const setBuffer = useCallback(
    (path: string, value: string) => {
      updateDoc(path, { buffer: value });
      scheduleSave(path, value);
    },
    [scheduleSave, updateDoc],
  );

  const registerCurrentDoc = useCallback((path: string | null) => {
    currentDocPathRef.current = path;
    setCurrentDocPathState(path);
  }, []);

  const requestJump = useCallback((path: string, line: number) => {
    setPendingJump({ path, line });
  }, []);

  const clearJump = useCallback(() => setPendingJump(null), []);

  const isGroupExpanded = useCallback((key: string) => groupExpanded[key] ?? true, [groupExpanded]);

  const toggleGroup = useCallback((key: string) => {
    setGroupExpanded((prev) => ({ ...prev, [key]: !(prev[key] ?? true) }));
  }, []);

  const openFlyout = useCallback((section: WorkspaceSection) => {
    if (flyoutTimer.current !== null) {
      clearTimeout(flyoutTimer.current);
      flyoutTimer.current = null;
    }
    setHoveredSection(section);
  }, []);

  const closeFlyoutNow = useCallback(() => {
    if (flyoutTimer.current !== null) {
      clearTimeout(flyoutTimer.current);
      flyoutTimer.current = null;
    }
    setHoveredSection(null);
  }, []);

  const scheduleCloseFlyout = useCallback(() => {
    if (flyoutTimer.current !== null) {
      clearTimeout(flyoutTimer.current);
    }
    flyoutTimer.current = setTimeout(() => setHoveredSection(null), FLYOUT_CLOSE_DELAY_MS);
  }, []);

  const sendChatMessage = useCallback(
    async (text: string) => {
      const userMsg: ChatMessage = { role: "user", content: text };
      const withUser = [...conversationRef.current, userMsg];
      conversationRef.current = withUser;
      setConversationState(withUser);

      const path = currentDocPathRef.current;
      const content = path !== null ? (docsRef.current[path]?.buffer ?? "") : null;

      const result = await postChat(sid, withUser, path, content);

      const assistantMsg: ChatMessage = { role: "assistant", content: result.reply_text ?? "" };
      const withReply = [...conversationRef.current, assistantMsg];
      conversationRef.current = withReply;
      setConversationState(withReply);

      if (result.edit !== null && path !== null && result.edit.path === path) {
        setBuffer(path, result.edit.new_content);
      }
    },
    [sid, setBuffer],
  );

  const getDoc = useCallback((path: string) => docsRef.current[path], []);

  const value: FilesContextValue = {
    sid,
    manifest,
    loading,
    getDoc,
    setBuffer,
    commitBufferNow,
    refetchTree,
    registerCurrentDoc,
    currentDocPath,
    pendingJump,
    requestJump,
    clearJump,
    chatOpen,
    setChatOpen,
    chatWidth,
    setChatWidth,
    conversation,
    sendChatMessage,
    isGroupExpanded,
    toggleGroup,
    hoveredSection,
    openFlyout,
    scheduleCloseFlyout,
    closeFlyoutNow,
    navigating,
    beginNavigation,
    endNavigation,
  };

  return <FilesContext.Provider value={value}>{children}</FilesContext.Provider>;
}

/**
 * The content derived views should read: the live buffer when it parses,
 * else the last-valid content (P10b — outline/derived views hold the last
 * valid state instead of breaking on a transient parse error).
 */
export function effectiveContent(doc: DocState | undefined): string {
  if (doc === undefined) {
    return "";
  }

  return isValidYAML(doc.buffer) ? doc.buffer : doc.lastValidContent;
}

export function useFiles(): FilesContextValue {
  const ctx = useContext(FilesContext);
  if (ctx === null) {
    throw new Error("useFiles must be used within a FilesProvider");
  }

  return ctx;
}
