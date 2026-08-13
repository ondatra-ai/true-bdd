"use client";

import { useState } from "react";
import Link from "next/link";
import TechPicker from "../../../components/TechPicker";
import EndpointsSection from "../../../components/EndpointsSection";

// Prototype-only: no persistence, in-memory React state, reset on reload.
const CATALOG = [
  "Go",
  "TypeScript",
  "Python",
  "net/http",
  "cobra",
  "Jest",
  "Playwright",
  "React",
  "Next.js",
  "Docker",
  "Postgres",
  "Redis",
  "gRPC",
  "GraphQL",
  "Kubernetes",
];

const SERVICE_TYPES = [
  { id: "custom", label: "Custom (developed here)" },
  { id: "postgres", label: "Postgres" },
  { id: "redis", label: "Redis" },
  { id: "mysql", label: "MySQL" },
  { id: "mongodb", label: "MongoDB" },
  { id: "rabbitmq", label: "RabbitMQ" },
];

// Supporting/infra types get a plausible default Connection card the moment
// you switch to them — still editable afterward, just seeded sensibly.
const CONNECTION_DEFAULTS = {
  postgres: { image: "postgres:16-alpine", port: "5432", volume: "pgdata:/var/lib/postgresql/data" },
  redis: { image: "redis:7-alpine", port: "6379", volume: "redisdata:/data" },
  mysql: { image: "mysql:8", port: "3306", volume: "mysqldata:/var/lib/mysql" },
  mongodb: { image: "mongo:7", port: "27017", volume: "mongodata:/data/db" },
  rabbitmq: { image: "rabbitmq:3-management", port: "5672", volume: "rabbitdata:/var/lib/rabbitmq" },
};

export default function ServicePage() {
  const [path, setPath] = useState("services/mcp");
  const [technologies, setTechnologies] = useState(["Go", "net/http"]);
  const [dependencies, setDependencies] = useState("none");
  const [serviceType, setServiceType] = useState("custom");

  const [qgPath, setQgPath] = useState("tests/integration");
  const [qgFramework, setQgFramework] = useState("Jest");
  const [qgConfig, setQgConfig] = useState("tests/jest.config.ts");

  const [connImage, setConnImage] = useState("");
  const [connPort, setConnPort] = useState("");
  const [connVolume, setConnVolume] = useState("");

  const isCustom = serviceType === "custom";

  function addTech(item) {
    setTechnologies((prev) => (prev.includes(item) ? prev : [...prev, item]));
  }
  function removeTech(item) {
    setTechnologies((prev) => prev.filter((t) => t !== item));
  }

  function handleTypeChange(e) {
    const id = e.target.value;
    setServiceType(id);
    const defaults = CONNECTION_DEFAULTS[id];
    if (defaults) {
      setConnImage(defaults.image);
      setConnPort(defaults.port);
      setConnVolume(defaults.volume);
    }
  }

  return (
    <>
      <nav
        className="mockup-breadcrumb"
        data-testid="mockup-breadcrumb"
        aria-label="Breadcrumb"
      >
        <Link href="/sessions">Sessions</Link>
        <span className="crumb-sep">/</span>
        <Link href="/workspace-overview">Workspace overview</Link>
        <span className="crumb-sep">/</span>
        <span aria-current="page">mcp-service</span>
      </nav>

      <main className="mockup-canvas canvas-max" data-testid="mockup-canvas">
        <div className="page-header">
          <div>
            <span className="section-label">01—Architecture / Services</span>
            <h1>mcp-service</h1>
            <p className="page-header__meta">docs/architecture/architecture.yaml</p>
          </div>
        </div>

        <dl className="run-meta-grid" style={{ gridTemplateColumns: "repeat(4, minmax(0,1fr))" }}>
          <div>
            <dt>Path</dt>
            <dd>
              <input
                className="inline-field-input"
                value={path}
                onChange={(e) => setPath(e.target.value)}
                aria-label="Path"
              />
            </dd>
          </div>

          <div>
            <dt>Type</dt>
            <dd>
              <select
                className="type-select"
                value={serviceType}
                onChange={handleTypeChange}
                aria-label="Service type"
              >
                {SERVICE_TYPES.map((t) => (
                  <option key={t.id} value={t.id}>
                    {t.label}
                  </option>
                ))}
              </select>
            </dd>
          </div>

          <div>
            <dt>Technologies</dt>
            <dd>
              <div className="tag-row tech-chip-row">
                {technologies.map((t) => (
                  <span className="chip" key={t}>
                    {t}
                    <button
                      type="button"
                      className="chip__remove"
                      aria-label={`Remove ${t}`}
                      onClick={() => removeTech(t)}
                    >
                      ×
                    </button>
                  </span>
                ))}
              </div>
              <TechPicker catalog={CATALOG} selected={technologies} onAdd={addTech} />
            </dd>
          </div>

          <div>
            <dt>Dependencies</dt>
            <dd>
              <input
                className="inline-field-input"
                value={dependencies}
                onChange={(e) => setDependencies(e.target.value)}
                aria-label="Dependencies"
              />
            </dd>
          </div>
        </dl>

        {isCustom ? (
          <>
            <h2 className="subsection">Quality gate — integration tests</h2>
            <dl
              className="run-meta-grid"
              style={{ gridTemplateColumns: "repeat(3, minmax(0,1fr))" }}
            >
              <div>
                <dt>Path</dt>
                <dd>
                  <input
                    className="inline-field-input"
                    value={qgPath}
                    onChange={(e) => setQgPath(e.target.value)}
                    aria-label="Quality gate path"
                  />
                </dd>
              </div>
              <div>
                <dt>Framework</dt>
                <dd>
                  <input
                    className="inline-field-input"
                    value={qgFramework}
                    onChange={(e) => setQgFramework(e.target.value)}
                    aria-label="Quality gate framework"
                  />
                </dd>
              </div>
              <div>
                <dt>Config</dt>
                <dd>
                  <input
                    className="inline-field-input"
                    value={qgConfig}
                    onChange={(e) => setQgConfig(e.target.value)}
                    aria-label="Quality gate config"
                  />
                </dd>
              </div>
            </dl>

            <EndpointsSection />

            <h2 className="subsection">Helpers</h2>
            <div className="card">
              <span className="section-label">McpClient</span>
              <p style={{ fontSize: "13px" }}>
                Typed JSON-RPC client that posts requests to the MCP endpoint and
                returns parsed responses.
              </p>
            </div>
          </>
        ) : (
          <>
            <h2 className="subsection">Connection</h2>
            <div className="card">
              <span className="section-label">
                Supporting service — no endpoints of its own
              </span>
              <dl
                className="run-meta-grid"
                style={{ gridTemplateColumns: "repeat(3, minmax(0,1fr))", marginTop: "10px" }}
              >
                <div>
                  <dt>Image</dt>
                  <dd>
                    <input
                      className="inline-field-input"
                      value={connImage}
                      onChange={(e) => setConnImage(e.target.value)}
                      aria-label="Connection image"
                    />
                  </dd>
                </div>
                <div>
                  <dt>Port</dt>
                  <dd>
                    <input
                      className="inline-field-input"
                      value={connPort}
                      onChange={(e) => setConnPort(e.target.value)}
                      aria-label="Connection port"
                    />
                  </dd>
                </div>
                <div>
                  <dt>Volume</dt>
                  <dd>
                    <input
                      className="inline-field-input"
                      value={connVolume}
                      onChange={(e) => setConnVolume(e.target.value)}
                      aria-label="Connection volume"
                    />
                  </dd>
                </div>
              </dl>
            </div>
          </>
        )}
      </main>
    </>
  );
}
