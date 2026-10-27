# ---- stage 1: build the React frontend ----
FROM node:20-alpine AS web
WORKDIR /app/web
COPY web/package.json ./
RUN npm install
COPY web/ ./
RUN npm run build

# ---- stage 2: build the static Go binary ----
FROM golang:1.22-alpine AS go
WORKDIR /app
COPY . .
# replace the placeholder dist with the real build
COPY --from=web /app/web/dist ./web/dist
RUN go mod tidy \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/chat .

# ---- stage 3: minimal runtime ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 chat
COPY --from=go /out/chat /usr/local/bin/chat
USER chat
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/chat"]