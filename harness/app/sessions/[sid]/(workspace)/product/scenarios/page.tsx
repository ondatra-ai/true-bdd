"use client";

// scenarios.yaml file page (P18): the requirements registry — a scenario IS
// the requirement entity of the workspace.

import { FileView } from "@/app/components/workspace/FileView";
import { SCENARIOS_PATH } from "@/app/lib/workspace/types";

export default function ScenariosPage() {
  return <FileView docPath={SCENARIOS_PATH} />;
}
