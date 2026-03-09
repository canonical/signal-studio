FROM node:22-alpine AS frontend-build
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm install
COPY frontend/ ./
RUN npm run build

FROM golang:1.25-alpine AS backend-build
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=frontend-build /app/frontend/dist ./cmd/server/static/
RUN CGO_ENABLED=0 go build -o /signal-studio ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=backend-build /signal-studio ./signal-studio
ENV SIGNAL_STUDIO_PORT=8080
EXPOSE 8080
ENTRYPOINT ["./signal-studio"]
