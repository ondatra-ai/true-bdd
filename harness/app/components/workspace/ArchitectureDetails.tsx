"use client";

/**
 * Per-service derived details on the architecture FILE PAGE (P15/P16):
 * tech stack + Docker provenance for every service, endpoints on custom
 * services, connection info on supporting ones — each region scoped to its
 * OWN service (a descendant of `arch-service-details[data-service=...]`),
 * absent from the other's.
 */

import { deriveArchitecture } from "@/app/lib/workspace/derive";
import { effectiveContent, useFiles } from "@/app/lib/workspace/files-context";
import { ARCHITECTURE_PATH } from "@/app/lib/workspace/types";

export function ArchitectureDetails() {
  const { getDoc } = useFiles();
  const content = effectiveContent(getDoc(ARCHITECTURE_PATH));
  const model = deriveArchitecture(content);

  return (
    <div className="ws-arch-details">
      {model.services.map((svc) => (
        <div key={svc.name} data-testid="arch-service-details" data-service={svc.name} className="ws-service-details">
          <h3>{svc.name}</h3>
          {svc.technologies.map((tech) => (
            <span key={tech} data-testid="service-tech" data-tech={tech} className="ws-chip">
              {tech}
            </span>
          ))}
          {svc.isCustom && svc.dockerfile !== undefined && (
            <div data-testid="service-docker-provenance" data-kind="dockerfile">
              {svc.dockerfile}
            </div>
          )}
          {!svc.isCustom && svc.composeRef !== undefined && (
            <div data-testid="service-docker-provenance" data-kind="compose_ref">
              {svc.composeRef}
            </div>
          )}
          {svc.isCustom &&
            svc.endpoints.map((ep, i) => (
              <div key={i} data-testid="service-endpoint" data-method={ep.method} data-path={ep.path} className="ws-endpoint">
                {ep.method} {ep.path}
                {ep.summary !== undefined ? ` — ${ep.summary}` : ""}
              </div>
            ))}
          {!svc.isCustom &&
            Object.entries(svc.connection).map(([key, value]) => (
              <div key={key} data-testid="service-connection" data-key={key} className="ws-connection">
                {key}: {String(value)}
              </div>
            ))}
        </div>
      ))}
    </div>
  );
}
