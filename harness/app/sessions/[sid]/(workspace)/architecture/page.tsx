"use client";

// Architecture spec file page (P8/P13/P15/P16): ONE GitHub-style file view
// over docs/architecture/architecture.yaml, plus the per-service derived
// details region (endpoints/tech/provenance).

import { ArchitectureDetails } from "@/app/components/workspace/ArchitectureDetails";
import { FileView } from "@/app/components/workspace/FileView";
import { ARCHITECTURE_PATH } from "@/app/lib/workspace/types";

export default function ArchitecturePage() {
  return (
    <div>
      <FileView docPath={ARCHITECTURE_PATH} />
      <ArchitectureDetails />
    </div>
  );
}
