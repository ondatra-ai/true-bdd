FROM node:20-bookworm-slim

WORKDIR /app

ENV NEXT_TELEMETRY_DISABLED=1
ENV NODE_ENV=production
ENV PORT=3000
ENV HOSTNAME=0.0.0.0

# No lockfile is committed, so `npm install` (not `npm ci`). NODE_ENV=production
# would normally omit devDependencies, but `next build` needs typescript and the
# @types packages, so include them explicitly.
COPY package.json ./
RUN npm install --include=dev --no-audit --no-fund

COPY . .

# Built with NODE_ENV=production, served with NODE_ENV=production: the artifact in
# .next matches what `next start` expects.
RUN npm run build

EXPOSE 3000

CMD ["npm", "run", "start"]
