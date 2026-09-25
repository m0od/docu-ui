.PHONY: web build test run image

web:
	cd web && npx -y pnpm@10 install --frozen-lockfile && npx -y pnpm@10 build
	touch web/dist/.gitkeep

build: web
	CGO_ENABLED=0 go build -trimpath -o bin/docu-ui ./cmd/docu-ui

test:
	go vet ./...
	go test -count=1 -cover ./...

run: build
	./bin/docu-ui

image:
	docker build -t docu-ui:dev .
