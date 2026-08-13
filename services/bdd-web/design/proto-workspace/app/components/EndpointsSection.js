"use client";

import { useState } from "react";

const METHODS = ["GET", "POST", "PUT", "DELETE", "PATCH"];

const INITIAL_ENDPOINTS = [
  {
    id: 1,
    method: "POST",
    path: "/mcp",
    summary: "JSON-RPC endpoint for tool calls",
    params: "Content-Type: application/json",
    request: `{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "search_docs",
    "arguments": { "query": "inventory spread" }
  }
}`,
    response: `{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [{ "type": "text", "text": "3 matches found" }]
  }
}`,
  },
  {
    id: 2,
    method: "GET",
    path: "/health",
    summary: "Liveness probe",
    params: "none",
    request: "(no body)",
    response: `{ "status": "ok" }`,
  },
  {
    id: 3,
    method: "GET",
    path: "/mcp/tools",
    summary: "List available tools",
    params: "none",
    request: "(no body)",
    response: `[
  { "name": "search_docs", "description": "Search the inventory docs" },
  { "name": "summarize", "description": "Summarize a shared doc" }
]`,
  },
];

let nextId = INITIAL_ENDPOINTS.length + 1;

// Swagger-ish endpoint list for a Custom service: solid method badge (reuses
// .status-pill — already a solid black/white block, no new color needed),
// path, one-line summary; each row is a native <details> so expand/collapse
// needs no React state. Adding is a tiny inline form; removing is a small
// "x" that stopPropagation-guards against also toggling the row open.
export default function EndpointsSection() {
  const [endpoints, setEndpoints] = useState(INITIAL_ENDPOINTS);
  const [newMethod, setNewMethod] = useState("GET");
  const [newPath, setNewPath] = useState("");
  const [newSummary, setNewSummary] = useState("");

  function removeEndpoint(id) {
    setEndpoints((prev) => prev.filter((e) => e.id !== id));
  }

  function addEndpoint() {
    const path = newPath.trim();
    if (!path) return;
    setEndpoints((prev) => [
      ...prev,
      {
        id: nextId++,
        method: newMethod,
        path: path.startsWith("/") ? path : `/${path}`,
        summary: newSummary.trim() || "—",
        params: "—",
        request: "(no example yet)",
        response: "(no example yet)",
      },
    ]);
    setNewPath("");
    setNewSummary("");
  }

  return (
    <>
      <h2 className="subsection">Endpoints</h2>
      <div className="endpoint-list">
        {endpoints.map((ep) => (
          <details className="endpoint-row" key={ep.id}>
            <summary className="endpoint-row__head">
              <span className="endpoint-caret" aria-hidden="true"></span>
              <span className="status-pill endpoint-method">{ep.method}</span>
              <code className="endpoint-path">{ep.path}</code>
              <span className="endpoint-summary">{ep.summary}</span>
              <button
                type="button"
                className="chip__remove endpoint-remove"
                aria-label={`Remove ${ep.method} ${ep.path}`}
                onClick={(e) => {
                  e.preventDefault();
                  e.stopPropagation();
                  removeEndpoint(ep.id);
                }}
              >
                ×
              </button>
            </summary>
            <div className="endpoint-detail">
              <dl className="endpoint-detail__meta">
                <div>
                  <dt>Params</dt>
                  <dd>{ep.params}</dd>
                </div>
              </dl>
              <h3 className="endpoint-detail__label">Example request</h3>
              <pre className="run-output endpoint-code">{ep.request}</pre>
              <h3 className="endpoint-detail__label">Example response</h3>
              <pre className="run-output endpoint-code">{ep.response}</pre>
            </div>
          </details>
        ))}
        {endpoints.length === 0 && (
          <p className="muted" style={{ fontSize: "13px", margin: "10px 0" }}>
            No endpoints yet.
          </p>
        )}
      </div>

      <div className="endpoint-add-row">
        <select
          value={newMethod}
          onChange={(e) => setNewMethod(e.target.value)}
          aria-label="New endpoint method"
        >
          {METHODS.map((m) => (
            <option key={m} value={m}>
              {m}
            </option>
          ))}
        </select>
        <input
          type="text"
          placeholder="/path"
          value={newPath}
          onChange={(e) => setNewPath(e.target.value)}
          aria-label="New endpoint path"
        />
        <input
          type="text"
          placeholder="One-line summary"
          value={newSummary}
          onChange={(e) => setNewSummary(e.target.value)}
          aria-label="New endpoint summary"
        />
        <button type="button" className="btn" onClick={addEndpoint} disabled={!newPath.trim()}>
          Add endpoint
        </button>
      </div>
    </>
  );
}
