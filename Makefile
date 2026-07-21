.PHONY: dev build frontend linux clean

dev:
	go run ./cmd/server

frontend:
	cd frontend && npm ci && npm run build

build: frontend
	rm -rf internal/static/dist && mkdir -p internal/static/dist
	cp -r frontend/dist/. internal/static/dist/
	go build -tags embedstatic -o dist/go-term ./cmd/server
	cp deploy/start.sh dist/start.sh && chmod +x dist/start.sh

linux-amd: frontend
	rm -rf internal/static/dist && mkdir -p internal/static/dist
	cp -r frontend/dist/. internal/static/dist/
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags embedstatic -o dist/go-term ./cmd/server

linux-arm: frontend
	rm -rf internal/static/dist && mkdir -p internal/static/dist
	cp -r frontend/dist/. internal/static/dist/
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags embedstatic -o dist/go-term ./cmd/server

macos-amd: frontend
	rm -rf internal/static/dist && mkdir -p internal/static/dist
	cp -r frontend/dist/. internal/static/dist/
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -tags embedstatic -o dist/go-term ./cmd/server

macos-arm: frontend
	rm -rf internal/static/dist && mkdir -p internal/static/dist
	cp -r frontend/dist/. internal/static/dist/
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -tags embedstatic -o dist/go-term ./cmd/server

clean:
	rm -rf dist frontend/dist internal/static/dist
