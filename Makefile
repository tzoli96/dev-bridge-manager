.PHONY: help dev build-backend build-frontend run-backend run-frontend stop-prod down logs shell clean

# Docker compose file
COMPOSE_FILE = .docker/docker-compose.yml

# Default target
help:
	@echo "🚀 Dev Bridge Manager - Available Commands:"
	@echo ""
	@echo "Development:"
	@echo "  make dev        - Start development environment"
	@echo "  make shell      - Enter backend container"
	@echo "  make logs       - Show logs"
	@echo "  make down       - Stop development containers"
	@echo ""
	@echo "Production Images:"
	@echo "  make build-backend   - Build backend production image"
	@echo "  make build-frontend  - Build frontend production image"
	@echo ""
	@echo "Production Run:"
	@echo "  make run-backend     - Run backend production container"
	@echo "  make run-frontend    - Run frontend production container"
	@echo "  make stop-prod       - Stop production containers"
	@echo ""
	@echo "Cleanup:"
	@echo "  make clean      - Remove everything"

# Development environment
dev:
	@echo "🏗️  Starting development environment..."
	docker compose -f $(COMPOSE_FILE) up -d

dev-build:
	@echo "🏗️  Building development environment..."
	docker compose -f $(COMPOSE_FILE) build

# Build production backend image
build-backend:
	@echo "🔨 Building backend production image..."
	docker build -f .docker/backend.Dockerfile -t devbridge-backend:latest .

# Build production frontend image
build-frontend:
	@echo "🔨 Building frontend production image..."
	docker build -f .docker/frontend.Dockerfile -t devbridge-frontend:latest .

# Run production backend
run-backend:
	@echo "🚀 Starting backend production container..."
	docker run -d --name devbridge-backend-prod -p 8080:8080 devbridge-backend:latest
	@echo "✅ Backend running on http://localhost:8080"

# Run production frontend
run-frontend:
	@echo "🚀 Starting frontend production container..."
	docker run -d --name devbridge-frontend-prod -p 3000:3000 devbridge-frontend:latest
	@echo "✅ Frontend running on http://localhost:3000"

# Stop production containers
stop-prod:
	@echo "🛑 Stopping production containers..."
	docker stop devbridge-backend-prod devbridge-frontend-prod || true
	docker rm devbridge-backend-prod devbridge-frontend-prod || true

# Stop development containers
down:
	@echo "🛑 Stopping development containers..."
	docker compose -f $(COMPOSE_FILE) down

# Show logs
logs:
	docker compose -f $(COMPOSE_FILE) logs -f

# Enter backend container
shell:
	docker compose -f $(COMPOSE_FILE) exec backend bash

# Clean everything
clean:
	@echo "🧹 Cleaning up..."
	docker compose -f $(COMPOSE_FILE) down -v --rmi all --remove-orphans
	docker stop devbridge-backend-prod devbridge-frontend-prod || true
	docker rm devbridge-backend-prod devbridge-frontend-prod || true
	docker rmi devbridge-backend:latest devbridge-frontend:latest || true ".PHONY: help up down build restart logs shell clean status