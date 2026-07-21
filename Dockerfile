# syntax=docker/dockerfile:1
# Multi-stage build for go-Term.
#   frontend : build the React/Vite app with node:22
#   backend  : compile a static linux/amd64 binary with golang:1.26
#   final    : minimal distroless image running the single binary

FROM node:22-bookworm AS frontend
WORKDIR /src
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.26 AS backend
ENV CGO_ENABLED=0
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/dist ./internal/static/dist
RUN go build -tags embedstatic -o /out/go-Term ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot AS final
COPY --from=backend /out/go-Term /go-Term
EXPOSE 8080
ENTRYPOINT ["/go-Term"]
