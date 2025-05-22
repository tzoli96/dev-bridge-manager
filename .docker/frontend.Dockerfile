# Development stage
FROM node:22-alpine AS development

RUN npm install -g pnpm
RUN apk add --no-cache curl bash

WORKDIR /app

# Copy package files and install dependencies
COPY frontend/package*.json ./
RUN pnpm install

# Copy source code
COPY frontend/ ./

EXPOSE 3000
CMD ["pnpm", "dev"]

# Build stage
FROM node:22-alpine AS builder

RUN npm install -g pnpm

WORKDIR /app

# Copy package files and install dependencies
COPY frontend/package*.json ./
RUN pnpm install

# Copy source and build
COPY frontend/ ./
RUN pnpm build

# Production stage
FROM node:22-alpine

RUN npm install -g pnpm && \
    adduser -D -s /bin/sh appuser

WORKDIR /app

# Copy built app and necessary files
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/package.json ./
COPY --from=builder /app/public ./public

# Install only production dependencies
RUN pnpm install --prod

RUN chown -R appuser:appuser .

USER appuser
EXPOSE 3000

CMD ["pnpm", "start"]