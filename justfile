set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

default:
    just --list

fmt:
    golangci-lint fmt

fmt-verify:
    golangci-lint fmt --diff

go-fix:
    go fix ./...

go-fix-verify:
    #!/usr/bin/env bash
    set -euo pipefail
    go fix ./...
    git diff --exit-code || (echo "go fix produced changes - run 'go fix ./...' locally and commit" && exit 1)

lint:
    go vet ./...
    golangci-lint run

lint-fix:
    just go-fix
    golangci-lint run --fix

vet:
    go vet ./...

test:
    go test ./...

test-ci:
    go test ./... -count=1
