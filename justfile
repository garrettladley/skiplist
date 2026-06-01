set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

default:
    just --list

fmt:
    golangci-lint fmt

fmt-verify:
    golangci-lint fmt --diff

lint:
    go vet ./...
    golangci-lint run

lint-fix:
    go fix ./...
    golangci-lint run --fix

vet:
    go vet ./...

test:
    go test ./...

test-ci:
    go test ./... -count=1
