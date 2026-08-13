import { StubPage } from "../../../components/Stub";

export const metadata = { title: "Builds — TrueBDD Workspace" };

// Builds is navigation-only for now (report F10): prod ships this section as a
// deliberate future-task stub, so the mockup matches that agreed scope rather
// than a fully-designed Runs list. The earlier Runs list + run-detail design
// (content/runs.html, content/run-detail.html, /run-detail) was removed with
// this change.
export default function Page() {
  return (
    <StubPage
      crumbs={[
        { label: "Sessions", href: "/sessions" },
        { label: "Workspace overview", href: "/workspace-overview" },
        { label: "Builds" },
      ]}
      kicker="04—Builds"
      title="Builds"
      meta="Build runs will live here in a future task. This section is navigation-only for now."
    />
  );
}
