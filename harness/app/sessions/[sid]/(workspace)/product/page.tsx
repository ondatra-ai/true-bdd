"use client";

// PRD file page = Product root (P18). Also hosts the "create a new story"
// entry point (P22) — required feature, existing-pick OR inline create.

import { useState } from "react";

import { FileView } from "@/app/components/workspace/FileView";
import { NewStoryForm } from "@/app/components/workspace/NewStoryForm";
import { PRD_PATH } from "@/app/lib/workspace/types";

export default function ProductPage() {
  const [creating, setCreating] = useState(false);

  return (
    <div>
      <FileView docPath={PRD_PATH} />
      <div style={{ marginTop: "1rem" }}>
        {!creating && (
          <button type="button" data-testid="new-story-open" onClick={() => setCreating(true)}>
            + New story
          </button>
        )}
        {creating && <NewStoryForm onClose={() => setCreating(false)} />}
      </div>
    </div>
  );
}
