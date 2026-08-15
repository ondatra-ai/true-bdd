import { test, expect } from '@playwright/test';

// INT-900 — MCP /mcp endpoint responds 200 to a valid initialize POST
// Service: mcp-service
// Given: the MCP server is running on its configured port
// When:  the Claude User posts a valid JSON-RPC initialize request to /mcp
// Then:  the server returns HTTP 200

// Given: the MCP server is running on its configured port.
// The base URL is taken from the environment so the test binds to whatever
// port the server was configured with, falling back to a conventional local port.
const MCP_BASE_URL = process.env.MCP_BASE_URL ?? 'http://127.0.0.1:3000';

test('INT-900: MCP /mcp endpoint responds 200 to a valid initialize POST', async ({ request }) => {
  // Given: the MCP server is running on its configured port
  const endpoint = `${MCP_BASE_URL}/mcp`;

  // When: the Claude User posts a valid JSON-RPC initialize request to /mcp
  const response = await request.post(endpoint, {
    headers: {
      'Content-Type': 'application/json',
      // MCP Streamable HTTP requires the client to accept both content types.
      Accept: 'application/json, text/event-stream',
    },
    data: {
      jsonrpc: '2.0',
      id: 1,
      method: 'initialize',
      params: {
        protocolVersion: '2025-06-18',
        capabilities: {},
        clientInfo: { name: 'claude-user', version: '1.0.0' },
      },
    },
  });

  // Then: the server returns HTTP 200
  expect(response.status()).toBe(200);
});
