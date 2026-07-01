set dotenv-load := true

addr := env_var_or_default("MINISWAR_ADDR", "127.0.0.1:8080")
db := env_var_or_default("MINISWAR_DB", "miniswar.sqlite")
gocache := env_var_or_default("GOCACHE", "/tmp/miniswar-go-build")
gomodcache := env_var_or_default("GOMODCACHE", "/tmp/miniswar-go-mod")
port := env_var_or_default("MINISWAR_PORT", "8080")
version_pkg := "miniswar/internal/version"
version := `cat internal/version/VERSION`
branch := `git rev-parse --abbrev-ref HEAD 2>/dev/null || true`
default_branch := `git symbolic-ref --short refs/remotes/origin/HEAD 2>/dev/null | sed 's#^origin/##' || true`
ldflags := "-X '" + version_pkg + ".baseVersion=" + version + "' -X '" + version_pkg + ".branchName=" + branch + "' -X '" + version_pkg + ".defaultBranch=" + default_branch + "'"

default:
    @just --list

test:
    GOCACHE={{gocache}} GOMODCACHE={{gomodcache}} go test ./...

build:
    GOCACHE={{gocache}} GOMODCACHE={{gomodcache}} go build -buildvcs=false -ldflags "{{ldflags}}" ./...

check: test build

run:
    GOCACHE={{gocache}} GOMODCACHE={{gomodcache}} go run -buildvcs=false -ldflags "{{ldflags}}" ./cmd/miniswar -addr {{addr}} -db {{db}}

run-local port:
    GOCACHE={{gocache}} GOMODCACHE={{gomodcache}} go run -buildvcs=false -ldflags "{{ldflags}}" ./cmd/miniswar -addr 10.0.10.23:{{port}} -db /tmp/miniswar-{{port}}.sqlite

run-port port:
    GOCACHE={{gocache}} GOMODCACHE={{gomodcache}} go run -buildvcs=false -ldflags "{{ldflags}}" ./cmd/miniswar -addr 127.0.0.1:{{port}} -db /tmp/miniswar-{{port}}.sqlite

fmt:
    gofmt -w cmd internal

clean:
    rm -f miniswar.sqlite /tmp/miniswar-*.sqlite
    rm -rf /tmp/miniswar-go-build

IMAGE_REGISTRY := "ghcr.io"
IMAGE_OWNER := "bkroeze"
IMAGE_NAME := "miniswar"
IMAGE_TAG := "latest"
IMAGE := IMAGE_REGISTRY + "/" + IMAGE_OWNER + "/" + IMAGE_NAME + ":" + IMAGE_TAG

# Build Docker image locally
docker-build:
    docker build -t {{IMAGE}} .

# Push the Docker image to GHCR (requires `docker login ghcr.io`)
docker-push:
    docker push {{IMAGE}}

# Run the Docker image on MINISWAR_PORT, defaulting to 8080.
docker-run:
    docker run --rm -p {{port}}:{{port}} {{IMAGE}} -addr 0.0.0.0:{{port}} -db /storage/miniswar.sqlite
