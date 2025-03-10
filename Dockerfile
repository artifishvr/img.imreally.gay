FROM oven/bun:1-alpine AS base
WORKDIR /app

# Install dependencies into temp directory for caching
FROM base AS install
WORKDIR /temp
COPY package.json bun.lock ./
# Install all dependencies for development
RUN bun install --frozen-lockfile
# Install production dependencies to a separate directory
WORKDIR /temp/prod-modules
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile --production

# Build the application
FROM base AS builder
WORKDIR /build
COPY --from=install /temp/node_modules ./node_modules
COPY . .
RUN bun run build

# Production image
FROM base AS release
# Create a non-root user for better security
RUN addgroup -S appuser && adduser -S -G appuser appuser
# Create directory for temp files
RUN mkdir -p /app/temp && chown -R appuser:appuser /app

# Copy only necessary files to production image
COPY --from=builder /build/.output/server .

ENV NODE_ENV=production
# Switch to non-root user for security
USER appuser

ENTRYPOINT ["bun", "run", "index.mjs"]

EXPOSE 3000