import { StubPage, StubRowList } from "../../../components/Stub";

export const metadata = { title: "Services — TrueBDD Workspace" };

const crumbs = [
  { href: "/sessions", label: "Sessions" },
  { href: "/workspace-overview", label: "Workspace overview" },
  { href: "/architecture", label: "Architecture" },
  { label: "Services" },
];

const items = [
  {
    href: "/service",
    title: "mcp-service",
    meta: "Go · net/http · services/mcp",
  },
];

export default function ServicesPage() {
  return (
    <StubPage
      crumbs={crumbs}
      kicker="01—Architecture / Services"
      title="Services"
      meta="Services declared in docs/architecture/architecture.yaml."
    >
      <StubRowList items={items} />
    </StubPage>
  );
}
