# syntax=docker/dockerfile:1

# 1. Web UI — node_modules live only in this stage
FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /src/web
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

# 2. Go binary with the built UI embedded
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS api
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/docu-ui ./cmd/docu-ui
# Empty dir so the runtime image gets a /data owned by the nonroot user (distroless has no mkdir)
RUN mkdir -p /out/data

# 3. Runtime — only the binary
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=api /out/docu-ui /docu-ui
COPY --from=api --chown=65532:65532 /out/data /data
VOLUME /data
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/docu-ui"]
