import { StubPage } from "../../../components/Stub";

export const metadata = { title: "docker-compose.yml — TrueBDD Workspace" };

const crumbs = [
  { href: "/sessions", label: "Sessions" },
  { href: "/workspace-overview", label: "Workspace overview" },
  { href: "/architecture", label: "Architecture" },
  { href: "/docker", label: "Docker" },
  { label: "docker-compose.yml" },
];

export default function DockerComposePage() {
  return (
    <StubPage
      crumbs={crumbs}
      kicker="01—Architecture / Docker"
      title="docker-compose.yml"
      meta="Terminal leaf — prototype stub only. This workspace has no docker-compose.yml authored yet; this page exists so the sidebar row has somewhere to navigate."
    >
      <p className="muted" style={{ fontSize: "13px" }}>
        Nothing to show yet. When Docker support is designed for real, this
        page would list services, ports, and volumes declared for the
        workspace&apos;s compose stack.
      </p>
    </StubPage>
  );
}
