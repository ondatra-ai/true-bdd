"use client";

/**
 * The per-section navigation tree (P13/P18a): rendered BOTH inside the
 * docked sidebar (the active section) and inside the rail's hover flyout
 * (a non-active section preview, P2) — the same component, two mount
 * points, so behavior (expand state, jumps) is identical either way.
 */

import type { ReactNode } from "react";
import { useEffect } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";

import { deriveArchitecture, deriveFeatures, deriveScenarios, deriveStory } from "@/app/lib/workspace/derive";
import { effectiveContent, useFiles } from "@/app/lib/workspace/files-context";
import { routes } from "@/app/lib/workspace/routes";
import { ARCHITECTURE_PATH, FEATURES_PATH, SCENARIOS_PATH, isStoryPath } from "@/app/lib/workspace/types";
import type { WorkspaceSection } from "@/app/lib/workspace/types";
import { lineOfMapKey, lineOfSeqItemByField } from "@/app/lib/workspace/yaml-utils";

function SidebarGroup({
  sectionKey,
  label,
  href,
  selected,
  children,
}: {
  sectionKey: string;
  label: string;
  href?: string;
  selected: boolean;
  children: ReactNode;
}) {
  const { isGroupExpanded, toggleGroup } = useFiles();
  const key = `${sectionKey}:${label}`;
  const expanded = isGroupExpanded(key);

  return (
    <div data-testid="sidebar-group" data-group={label} className="ws-sidebar-group">
      <div className="ws-sidebar-group-header">
        <button
          type="button"
          data-testid="sidebar-caret"
          data-expanded={String(expanded)}
          className="ws-sidebar-caret"
          onClick={(event) => {
            event.preventDefault();
            event.stopPropagation();
            toggleGroup(key);
          }}
        >
          {expanded ? "▾" : "▸"}
        </button>
        {href !== undefined ? (
          <Link
            data-testid="sidebar-group-name"
            data-selected={String(selected)}
            href={href}
            scroll={false}
            className="ws-sidebar-group-name"
          >
            {label}
          </Link>
        ) : (
          <span data-testid="sidebar-group-name" data-selected={String(selected)} className="ws-sidebar-group-name">
            {label}
          </span>
        )}
      </div>
      <div data-testid="sidebar-group-body" className="ws-sidebar-group-body" style={{ display: expanded ? undefined : "none" }}>
        <div data-testid="sidebar-guide-line" className="ws-guide-line" />
        {children}
      </div>
    </div>
  );
}

function ArchitectureTree({ sectionKey }: { sectionKey: string }) {
  const { sid, getDoc, requestJump, beginNavigation } = useFiles();
  const pathname = usePathname();
  const router = useRouter();

  const content = effectiveContent(getDoc(ARCHITECTURE_PATH));
  const model = deriveArchitecture(content);

  // Prefetch the architecture route so a cross-page click (from a story page,
  // via the flyout — w3.3) is an INSTANT client-side swap, not a race against
  // an in-flight RSC fetch (Next shows the PREVIOUS page's content until the
  // new route's payload arrives, which would otherwise let a read/write
  // immediately after `.click()` observe stale content).
  useEffect(() => {
    router.prefetch(routes.architecture(sid));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sid]);

  function jump(line: number) {
    requestJump(ARCHITECTURE_PATH, line);
    const target = routes.architecture(sid);
    if (pathname !== target) {
      beginNavigation();
      router.push(target, { scroll: false });
    }
  }

  return (
    <>
      <SidebarGroup sectionKey={sectionKey} label="Services" selected={false}>
        {model.services.map((svc) => (
          <div
            key={svc.name}
            data-testid="arch-service-row"
            data-service={svc.name}
            className="ws-row"
            onClick={() => jump(lineOfMapKey(content, ["services", svc.name]) ?? 0)}
          >
            {svc.name}
          </div>
        ))}
      </SidebarGroup>
      <SidebarGroup sectionKey={sectionKey} label="Terms" selected={false}>
        {model.terms.map((term) => (
          <div
            key={term.name}
            data-testid="arch-term-row"
            data-term={term.name}
            className="ws-row"
            onClick={() => jump(lineOfSeqItemByField(content, ["terms"], "name", term.name) ?? 0)}
          >
            {term.name}
          </div>
        ))}
      </SidebarGroup>
      <SidebarGroup sectionKey={sectionKey} label="Docker" selected={false}>
        {model.composeFile !== null && (
          <div
            data-testid="arch-docker-row"
            className="ws-row"
            onClick={() => jump(lineOfMapKey(content, ["docker", "compose_file"]) ?? 0)}
          >
            {model.composeFile}
          </div>
        )}
      </SidebarGroup>
    </>
  );
}

function ProductTree({ sectionKey }: { sectionKey: string }) {
  const { sid, manifest, getDoc, requestJump, beginNavigation } = useFiles();
  const pathname = usePathname();
  const router = useRouter();

  const prdRoute = routes.product(sid);
  const featuresRoute = routes.features(sid);
  const scenariosRoute = routes.scenarios(sid);

  const featuresContent = effectiveContent(getDoc(FEATURES_PATH));
  const features = deriveFeatures(featuresContent);

  const scenariosContent = effectiveContent(getDoc(SCENARIOS_PATH));
  const scenarios = deriveScenarios(scenariosContent);

  const storyPaths = manifest.filter((entry) => isStoryPath(entry.path)).map((entry) => entry.path);
  const stories = storyPaths
    .map((path) => {
      const info = deriveStory(effectiveContent(getDoc(path)));

      return info !== null ? { ...info, path } : null;
    })
    .filter((s): s is { id: string; title?: string; feature?: string; path: string } => s !== null)
    .sort((a, b) => a.id.localeCompare(b.id, undefined, { numeric: true }));

  const storyIds = stories.map((s) => s.id).join(",");

  // Prefetch every cross-page navigation target this tree can jump to (same
  // reasoning as ArchitectureTree above) — a story row's navigation must be
  // instant, since the test reads/writes the editor immediately after the
  // click with no wait for the route swap to settle.
  const featureIdsJoined = features.map((f) => f.id).join(",");

  useEffect(() => {
    router.prefetch(prdRoute);
    router.prefetch(featuresRoute);
    router.prefetch(scenariosRoute);
    for (const story of stories) {
      router.prefetch(routes.story(sid, story.id));
    }
    for (const feature of features) {
      router.prefetch(routes.feature(sid, feature.id));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sid, storyIds, featureIdsJoined]);

  function gotoStory(path: string, storyId: string) {
    requestJump(path, 0);
    const target = routes.story(sid, storyId);
    if (pathname !== target) {
      beginNavigation();
      router.push(target, { scroll: false });
    }
  }

  function gotoScenario(id: string) {
    requestJump(SCENARIOS_PATH, lineOfMapKey(scenariosContent, ["scenarios", id]) ?? 0);
    if (pathname !== scenariosRoute) {
      beginNavigation();
      router.push(scenariosRoute, { scroll: false });
    }
  }

  return (
    <>
      <SidebarGroup sectionKey={sectionKey} label="PRD" href={prdRoute} selected={pathname === prdRoute}>
        <Link
          data-testid="prd-row"
          href={prdRoute}
          scroll={false}
          data-selected={String(pathname === prdRoute)}
          className="ws-row"
        >
          PRD
        </Link>
      </SidebarGroup>
      <SidebarGroup sectionKey={sectionKey} label="Features" href={featuresRoute} selected={pathname === featuresRoute}>
        {features.map((feature) => {
          const featureRoute = routes.feature(sid, feature.id);

          return (
            <Link
              key={feature.id}
              data-testid="feature-row"
              data-feature={feature.id}
              data-selected={String(pathname === featureRoute)}
              href={featureRoute}
              scroll={false}
              className="ws-row"
            >
              {feature.id}
            </Link>
          );
        })}
      </SidebarGroup>
      <SidebarGroup sectionKey={sectionKey} label="Stories" selected={false}>
        {stories.map((story) => (
          <div
            key={story.path}
            data-testid="story-row"
            data-story-id={story.id}
            data-selected={String(pathname === routes.story(sid, story.id))}
            className="ws-row"
            onClick={() => gotoStory(story.path, story.id)}
          >
            {story.id}
            {story.title !== undefined ? ` — ${story.title}` : ""}
          </div>
        ))}
      </SidebarGroup>
      <SidebarGroup sectionKey={sectionKey} label="Scenarios" href={scenariosRoute} selected={pathname === scenariosRoute}>
        {scenarios.map((scenario) => (
          <div
            key={scenario.id}
            data-testid="scenario-row"
            data-scenario-id={scenario.id}
            data-selected={String(pathname === scenariosRoute)}
            className="ws-row"
            onClick={() => gotoScenario(scenario.id)}
          >
            {scenario.id}
          </div>
        ))}
      </SidebarGroup>
    </>
  );
}

function HomeTree() {
  return <div className="ws-empty-tree">Workspace overview — no single backing file.</div>;
}

function BuildsTree() {
  return <div className="ws-empty-tree">Builds — navigation-only for now.</div>;
}

export function SectionTree({ section }: { section: WorkspaceSection }) {
  switch (section) {
    case "architecture":
      return <ArchitectureTree sectionKey={section} />;
    case "product":
      return <ProductTree sectionKey={section} />;
    case "builds":
      return <BuildsTree />;
    case "home":
    default:
      return <HomeTree />;
  }
}
