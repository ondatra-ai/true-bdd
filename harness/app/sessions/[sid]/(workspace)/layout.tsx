// Persistent workspace route-group layout (P1–P12): the shell (FilesProvider,
// icon rail, docked sidebar, docked chat, 100vh app-shell scroll frame)
// SURVIVES client-side navigation across every `(workspace)` page for one
// session (Established fact — a persistent route-group layout keeps its
// component state across nav). A thin Server Component that just resolves
// `sid` and hands off to the client `WorkspaceShell`.

import type { ReactNode } from "react";

import { WorkspaceShell } from "@/app/components/workspace/WorkspaceShell";

export default async function WorkspaceLayout({
  children,
  params,
}: {
  children: ReactNode;
  params: Promise<{ sid: string }>;
}): Promise<ReactNode> {
  const { sid } = await params;

  return <WorkspaceShell sid={sid}>{children}</WorkspaceShell>;
}
