.PHONY: web build run dev clean

# Build the React frontend into web/dist
web:
	cd web && npm install && npm run build

# Build the Go binary (embeds web/dist) — requires `go` and `make web` first
build: web
	CGO_ENABLED=0 go build -trimpath -o chat .

# Local dev: run backend while `npm run dev` serves the frontend on :5173
run:
	go run .

# Everything for deployment inside Docker (used by deploy/docker-compose.yml)
clean:
	rm -f chat
	rm -rf web/node_modules web/dist