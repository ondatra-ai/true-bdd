"use client";

import { useMemo } from "react";
import { usePathname } from "next/navigation";
import { NavLink, CaretName } from "./nav-link";
import {
  useArchitectureYaml,
  parseArchitectureOutline,
  ARCH_TOP_ID,
  ARCH_SERVICES_ID,
  ARCH_TERMS_ID,
  ARCH_DOCKER_ID,
  ARCH_DOCKER_COMPOSE_ID,
} from "./ArchitectureYamlContext";
import { useFiles } from "./FilesStore";
import { useProductOutline, FILE_TOP_ID, storySlugFromId } from "./ProductFiles";
import NewStoryForm from "./NewStoryForm";

// Four ClickUp-style rail sections. `match` decides which section a route
// belongs to (see components/Rail.js + DockedPanel.js for how this drives
// active-rail-highlight + which tree is docked/flown out).
export const SECTIONS = [
  {
    id: "home",
    label: "Home",
    icon: "⌂", // ⌂
    href: "/workspace-overview",
    match: (p) => p === "/workspace-overview",
  },
  {
    id: "architecture",
    label: "Architecture",
    icon: "▦", // ▦
    href: "/architecture",
    match: (p) =>
      [
        "/architecture",
        "/services",
        "/service",
        "/terms",
        "/vocabulary",
        "/docker",
        "/docker-compose",
      ].includes(p),
  },
  {
    id: "product",
    label: "Product",
    icon: "◆", // ◆
    href: "/product",
    match: (p) =>
      p.startsWith("/story/") ||
      p.startsWith("/feature/") ||
      [
        "/product",
        "/product-overview",
        "/epic",
        "/features",
        "/story-detail",
        "/story-ambiguous",
        "/story-invalid",
        "/scenario-detail",
        "/scenarios",
        "/requirements",
      ].includes(p),
  },
  {
    id: "builds",
    label: "Builds",
    icon: "▶", // ▶
    href: "/runs",
    match: (p) =>
      ["/runs", "/prompt-choice", "/prompt-clarify", "/prompt-freetext"].includes(p),
  },
];

// /unavailable (a global degraded-state page) matches no section; falls
// back to Home so the docked panel always has something sane to show.
const FALLBACK_ID = "home";

export function useActiveSection() {
  const pathname = usePathname();
  const found = SECTIONS.find((s) => s.match(pathname));
  return found ? found.id : null;
}

export function useDockedSectionId() {
  return useActiveSection() || FALLBACK_ID;
}

function HomeTree() {
  return (
    <NavLink href="/workspace-overview" className="sidebar-flat-link">
      Workspace overview
    </NavLink>
  );
}

function ArchitectureTree() {
  // Architecture is now ONE file view (/architecture, architecture.yaml
  // rendered whole). The THREE GROUPS below (Services:/Terms:/Docker:) are
  // the file's mandatory schema sections — fixed, hardcoded, always
  // present. The ROWS inside each group are NOT hardcoded: they're read
  // live from the current YAML (shared context), so adding "redis" via
  // chat — or typing a new service into the textarea by hand — makes a row
  // appear here in real time. Clicking any row requests an exact-line jump
  // in the file view (handled by the /architecture page itself); the old
  // form-based pages (/services, /service, /terms, /docker,
  // /docker-compose) still exist and still work, just aren't linked here.
  const { yaml, requestJump } = useArchitectureYaml();
  const outline = useMemo(() => parseArchitectureOutline(yaml), [yaml]);

  // Deliberately NOT preventDefault: the Link must still navigate normally
  // (this row might be showing in a rail flyout while browsing a totally
  // different section — clicking it needs to both jump to /architecture
  // AND land you there). requestJump just piggybacks on the same click.
  function jump(id) {
    return () => requestJump(id);
  }

  return (
    <details className="sidebar-section" data-testid="sidebar-section-architecture" open>
      <summary>
        <CaretName href="/architecture" scroll={false} onNavigate={() => requestJump(ARCH_TOP_ID)}>
          01—Architecture
        </CaretName>
      </summary>

      <details className="sidebar-epic" open>
        <summary>
          <CaretName
            href="/architecture"
            scroll={false}
            disableActive
            onNavigate={() => requestJump(ARCH_SERVICES_ID)}
          >
            Services:
          </CaretName>
        </summary>
        <ul className="sidebar-tree" data-testid="arch-services-list">
          {outline.services.map((svc) => (
            <li key={svc.id}>
              <NavLink
                href="/architecture"
                scroll={false}
                disableActive
                onClick={jump(svc.id)}
                data-testid="arch-service-row"
              >
                {svc.key} <span className="sidebar-row-service">{svc.key === "mcp-service" ? "service" : "image"}</span>
              </NavLink>
            </li>
          ))}
          {outline.services.length === 0 && (
            <li>
              <span className="sidebar-tree__label">No services in file</span>
            </li>
          )}
        </ul>
      </details>

      <details className="sidebar-epic" open>
        <summary>
          <CaretName
            href="/architecture"
            scroll={false}
            disableActive
            onNavigate={() => requestJump(ARCH_TERMS_ID)}
          >
            Terms:
          </CaretName>
        </summary>
        <ul className="sidebar-tree" data-testid="arch-terms-list">
          {outline.terms.map((term) => (
            <li key={term.id}>
              <NavLink
                href="/architecture"
                scroll={false}
                disableActive
                onClick={jump(term.id)}
                data-testid="arch-term-row"
              >
                {term.name}
              </NavLink>
            </li>
          ))}
          {outline.terms.length === 0 && (
            <li>
              <span className="sidebar-tree__label">No terms in file</span>
            </li>
          )}
        </ul>
      </details>

      <details className="sidebar-epic" open>
        <summary>
          <CaretName
            href="/architecture"
            scroll={false}
            disableActive
            onNavigate={() => requestJump(ARCH_DOCKER_ID)}
          >
            Docker:
          </CaretName>
        </summary>
        <ul className="sidebar-tree" data-testid="arch-docker-list">
          {outline.dockerCompose ? (
            <li>
              <NavLink
                href="/architecture"
                scroll={false}
                disableActive
                onClick={jump(ARCH_DOCKER_COMPOSE_ID)}
                data-testid="arch-docker-compose-row"
              >
                {outline.dockerCompose.path}
              </NavLink>
            </li>
          ) : (
            <li>
              <span className="sidebar-tree__label">No compose_file in file</span>
            </li>
          )}
        </ul>
      </details>
    </details>
  );
}

function ProductTree() {
  // Same treatment as Architecture: product document / Features: / Stories: / Scenarios:
  // are the file-backed schema's FIXED, mandatory groups (hardcoded), but
  // every ROW inside them is read live from the current file contents
  // (shared FilesStore) — adding a story/scenario/feature via chat or the
  // new-story form, or editing a file's textarea by hand, makes a row
  // appear here immediately. There is no epic layer: Stories: is a FLAT
  // list of every docs/product/stories/*.yaml file (derived straight from the
  // store, see ProductFiles.deriveStories), direct children of Product.
  // Features: is the new alignment tag — features.yaml only holds
  // id+description, so each row IS the feature's id, linking to its own
  // /feature/<id> DERIVED aggregation page (not a file). Clicking a
  // story/scenario row navigates to that row's OWN file page and requests
  // an exact-line jump that the target page's FileView picks up once
  // mounted (jumpRequest lives in the provider, which never unmounts
  // across the navigation). The old form-based pages (/product-overview,
  // /story-detail, /scenario-detail, /scenarios) still exist and still
  // work, just aren't linked here.
  const { requestJump } = useFiles();
  const outline = useProductOutline();

  function jump(id) {
    return () => requestJump(id);
  }

  return (
    <details className="sidebar-section" data-testid="sidebar-section-product" open>
      <summary>
        <CaretName href="/product" scroll={false} onNavigate={() => requestJump(FILE_TOP_ID)}>
          02—Product
        </CaretName>
      </summary>

      <ul className="sidebar-tree">
        <li>
          <NavLink href="/product" scroll={false} onClick={jump(FILE_TOP_ID)}>
            product document
          </NavLink>
        </li>
      </ul>

      <details className="sidebar-epic" open>
        <summary>
          <CaretName
            href="/features"
            scroll={false}
            disableActive
            onNavigate={() => requestJump(FILE_TOP_ID)}
          >
            Features:
          </CaretName>
        </summary>
        <div className="sidebar-tree sidebar-tree--flat" data-testid="features-list">
          {outline.featuresOutline.features.map((f) => (
            <NavLink key={f.id} href={`/feature/${f.id}`} scroll={false} data-testid="feature-row">
              {f.id}
            </NavLink>
          ))}
          {outline.featuresOutline.features.length === 0 && (
            <span className="sidebar-tree__label">No features in file</span>
          )}
        </div>
      </details>

      <details className="sidebar-epic" open>
        <summary>
          <CaretName
            href="/product"
            scroll={false}
            disableActive
            onNavigate={() => requestJump(FILE_TOP_ID)}
          >
            Stories:
          </CaretName>
        </summary>
        <div className="sidebar-tree sidebar-tree--flat" data-testid="stories-list">
          {outline.stories.map((s) => (
            <NavLink
              key={s.id}
              href={`/story/${storySlugFromId(s.id)}`}
              scroll={false}
              onClick={jump(FILE_TOP_ID)}
              data-testid="story-row"
            >
              {s.id} — {s.title}
            </NavLink>
          ))}
          {outline.stories.length === 0 && (
            <span className="sidebar-tree__label">No story files</span>
          )}
          <NewStoryForm />
        </div>
      </details>

      {/* Requirements/Scenarios nests inside Product, per earlier steering:
          "Product (should include Requirements)". */}
      <details className="sidebar-epic" open>
        <summary>
          <CaretName href="/requirements" scroll={false} onNavigate={() => requestJump(FILE_TOP_ID)}>
            Scenarios:
          </CaretName>
        </summary>
        <div className="sidebar-tree sidebar-tree--flat" data-testid="scenarios-list">
          {outline.scenariosOutline.scenarios.map((s) => (
            <NavLink
              key={s.id}
              data-testid="scenario-row"
              href="/requirements"
              scroll={false}
              onClick={jump(s.anchorId)}
            >
              {s.id} <span className="sidebar-row-service">mcp-service</span>
            </NavLink>
          ))}
          {outline.scenariosOutline.scenarios.length === 0 && (
            <span className="sidebar-tree__label">No scenarios in file</span>
          )}
        </div>
      </details>
    </details>
  );
}

function BuildsTree() {
  // Builds is navigation-only for now (report F10): prod ships this as a
  // future-task stub, so the mockup mirrors that — the header points at the
  // /runs stub and there's no runs list to enumerate yet.
  return (
    <details className="sidebar-section" data-testid="sidebar-section-builds" open>
      <summary>
        <CaretName href="/runs">04—Builds</CaretName>
      </summary>
      <ul className="sidebar-tree">
        <li>
          <span className="sidebar-tree__label">Build runs — a future task</span>
        </li>
      </ul>
    </details>
  );
}

const TREES = {
  home: HomeTree,
  architecture: ArchitectureTree,
  product: ProductTree,
  builds: BuildsTree,
};

export function SectionTree({ id }) {
  const Tree = TREES[id] || HomeTree;
  return <Tree />;
}
