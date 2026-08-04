"use client";

// features.yaml file page (P18/P18a): id+description-only records (P20);
// free-text editing stays YAML-gated only (never schema-policed — P9).

import { FileView } from "@/app/components/workspace/FileView";
import { FEATURES_PATH } from "@/app/lib/workspace/types";

export default function FeaturesPage() {
  return <FileView docPath={FEATURES_PATH} />;
}
