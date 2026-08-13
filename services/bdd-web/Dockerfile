# Harness (Next.js relay + UI) production image.
#
# Built once by the e2e suite (global-setup: `docker compose build harness`) and
# by the dev stack (`docker compose up`). Uses Next.js standalone output
# (next.config.ts `output: "standalone"`) so the runtime layer is just the traced
# server + static assets — no full node_modules, no `next` CLI.
#
# Node pinned to the harness .nvmrc line (26.x). The image serves on PORT (4517)
# bound to 0.0.0.0 so a published host port reaches it.

# --- deps: install with the committed lockfile ---
FROM node:26-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

# --- build: produce .next/standalone ---
FROM node:26-alpine AS build
WORKDIR /app
ENV NEXT_TELEMETRY_DISABLED=1
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

# --- runtime: standalone server only ---
FROM node:26-alpine AS runtime
WORKDIR /app
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    HOSTNAME=0.0.0.0 \
    PORT=4517
# Standalone bundle ships its own minimal node_modules + server.js; static
# assets are not traced and must be copied alongside.
COPY --from=build /app/.next/standalone ./
COPY --from=build /app/.next/static ./.next/static
EXPOSE 4517
CMD ["node", "server.js"]
