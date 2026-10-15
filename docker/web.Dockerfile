FROM node:22-alpine AS builder

WORKDIR /app

# Install pnpm
RUN corepack enable && corepack prepare pnpm@latest --activate

# Copy workspace configuration files (no root package.json — pnpm works without it)
COPY pnpm-workspace.yaml turbo.json ./
COPY pnpm-lock.yaml ./

# Copy all packages (needed for workspace dependencies)
COPY packages/ ./packages/

# Copy the web app
COPY apps/web/ ./apps/web/

# Install dependencies (frozen lockfile ensures reproducibility)
RUN pnpm install --frozen-lockfile

# Build the web app (package name is garda-inflasi-web, not web)
RUN pnpm --filter garda-inflasi-web build

# ── Runtime stage ──────────────────────────────────────────────────────
FROM node:22-alpine

RUN apk add --no-cache tzdata

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy standalone output
COPY --from=builder /app/apps/web/.next/standalone ./apps/web/
COPY --from=builder /app/apps/web/.next/static ./apps/web/.next/static
COPY --from=builder /app/apps/web/public ./apps/web/public

# Use non-root user
USER appuser

EXPOSE 3000

ENV NODE_ENV=production
ENV HOSTNAME=0.0.0.0

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:3000/ || exit 1

CMD ["node", "apps/web/server.js"]

