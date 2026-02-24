set dotenv-load

export PATH := home_directory() / "go/bin:" + env("PATH")

# List available recipes
default:
    @just --list

# Run backend and frontend concurrently
dev:
    just backend & just frontend & wait

# Run Go backend with hot-reload via air
backend:
    cd backend && air

# Run Vite dev server
frontend:
    cd frontend && npm run dev

# Run all tests
test:
    cd backend && go test ./...

# Build everything for production
build:
    cd frontend && npm run build
    cd backend && go build -o ./tmp/server ./cmd/server

# Type-check frontend
typecheck:
    cd frontend && npx tsc --noEmit
