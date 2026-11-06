# syntax=docker/dockerfile:1
FROM node:22-alpine AS builder

WORKDIR /app

# Install pnpm (pin to v10 for lockfile compatibility)
RUN corepack enable && corepack prepare pnpm@10 --activate

# Set CI to avoid interactive prompts
ENV CI=true
# Store pnpm di cache mount BuildKit (bertahan antar build): lockfile yang berubah hanya mengunduh paket
# yang belum ada, dan unduhan yang putus di jaringan lambat diulang lebih sabar.
ENV npm_config_store_dir=/pnpm/store npm_config_fetch_retries=5 npm_config_fetch_retry_maxtimeout=120000

# Copy workspace configuration files (no root package.json — pnpm works without it)
COPY pnpm-workspace.yaml turbo.json ./
COPY pnpm-lock.yaml ./

# Unduh paket hanya dari lockfile. Layer ini tetap di cache selama pnpm-lock.yaml tidak
# berubah, sehingga perubahan kode tidak memicu unduh ulang seluruh paket npm.
RUN --mount=type=cache,id=garda-pnpm-store,target=/pnpm/store pnpm fetch --frozen-lockfile

# Copy all packages (needed for workspace dependencies)
COPY packages/ ./packages/

# Copy the web app
COPY apps/web/ ./apps/web/

# Public (browser-visible) configuration. These MUST be declared as ARG and
# promoted to ENV before `next build`, otherwise Next.js cannot inline them
# into the client bundle and the app silently falls back to mock mode.
ARG NEXT_PUBLIC_API_BASE_URL
ARG NEXT_PUBLIC_APP_URL
ARG NEXT_PUBLIC_MAPBOX_TOKEN
ENV NEXT_PUBLIC_API_BASE_URL=$NEXT_PUBLIC_API_BASE_URL
ENV NEXT_PUBLIC_APP_URL=$NEXT_PUBLIC_APP_URL
ENV NEXT_PUBLIC_MAPBOX_TOKEN=$NEXT_PUBLIC_MAPBOX_TOKEN

# Pasang dependensi dari store hasil `pnpm fetch` (tanpa jaringan)
RUN --mount=type=cache,id=garda-pnpm-store,target=/pnpm/store pnpm install --offline --frozen-lockfile

# Build the web app (package name is garda-inflasi-web, not web)
RUN pnpm --filter garda-inflasi-web build

# ── Runtime stage ──────────────────────────────────────────────────────
FROM node:22-alpine

RUN apk add --no-cache tzdata

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy standalone output
COPY --from=builder /app/apps/web/.next/standalone ./
COPY --from=builder /app/apps/web/.next/static ./apps/web/.next/static
COPY --from=builder /app/apps/web/public ./apps/web/public

# Use non-root user
USER appuser

EXPOSE 3000

ENV NODE_ENV=production
ENV HOSTNAME=0.0.0.0

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://127.0.0.1:3000/ || exit 1

CMD ["node", "apps/web/server.js"]

