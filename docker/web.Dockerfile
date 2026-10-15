FROM node:20-alpine AS builder

WORKDIR /app

# Copy workspace config
COPY pnpm-workspace.yaml package.json ./
COPY turbo.json ./

# Copy web app
COPY apps/web/ ./apps/web/

# Install dependencies
RUN corepack enable && pnpm install --frozen-lockfile

# Build
RUN pnpm --filter web build

# Runtime stage
FROM node:20-alpine

WORKDIR /app

COPY --from=builder /app/apps/web/.next/standalone ./apps/web/
COPY --from=builder /app/apps/web/.next/static ./apps/web/.next/static
COPY --from=builder /app/apps/web/public ./apps/web/public

EXPOSE 3000

CMD ["node", "apps/web/server.js"]
