.PHONY: openapi-generate openapi-check test run-gin

openapi-generate:
	./scripts/generate_openapi.sh

openapi-check:
	./scripts/check_openapi_sync.sh

test:
	go test ./...

run-gin:
	go run ./cmd/api
