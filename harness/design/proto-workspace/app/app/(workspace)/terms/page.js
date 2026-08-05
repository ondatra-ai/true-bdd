import { StubPage, StubRowList } from "../../../components/Stub";

export const metadata = { title: "Terms — TrueBDD Workspace" };

const crumbs = [
  { href: "/sessions", label: "Sessions" },
  { href: "/workspace-overview", label: "Workspace overview" },
  { href: "/architecture", label: "Architecture" },
  { label: "Terms" },
];

const items = [
  {
    href: "/vocabulary",
    title: "Vocabulary",
    meta: "Shared BDD glossary terms referenced by scenarios and specs",
  },
];

export default function TermsPage() {
  return (
    <StubPage
      crumbs={crumbs}
      kicker="01—Architecture / Terms"
      title="Terms"
      meta="Shared vocabulary referenced across the architecture spec."
    >
      <StubRowList items={items} />
    </StubPage>
  );
}
