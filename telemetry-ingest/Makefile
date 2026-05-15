GO_IMAGE ?= golang:1.25-alpine
REPO_ROOT ?= $(abspath ..)

.PHONY: test

test:
	@if command -v go >/dev/null 2>&1; then \
		go test ./...; \
	else \
		docker run --rm -v "$(REPO_ROOT)":/repo -w /repo/telemetry-ingest $(GO_IMAGE) go test ./...; \
	fi
