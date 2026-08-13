import { StubPage, StubRowList } from "../../../components/Stub";

export const metadata = { title: "Docker — TrueBDD Workspace" };

const crumbs = [
  { href: "/sessions", label: "Sessions" },
  { href: "/workspace-overview", label: "Workspace overview" },
  { href: "/architecture", label: "Architecture" },
  { label: "Docker" },
];

const items = [
  {
    href: "/docker-compose",
    title: "docker-compose.yml",
    meta: "stub — no compose file authored yet",
  },
];

export default function DockerPage() {
  return (
    <StubPage
      crumbs={crumbs}
      kicker="01—Architecture / Docker"
      title="Docker"
      meta="Local infra for this workspace. New group added while prototyping the sidebar hierarchy — no real Docker assets exist yet."
    >
      <StubRowList items={items} />
    </StubPage>
  );
}
