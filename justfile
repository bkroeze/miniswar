set dotenv-load := true

addr := env_var_or_default("MINISWAR_ADDR", "127.0.0.1:8080")
db := env_var_or_default("MINISWAR_DB", "miniswar.sqlite")
gocache := env_var_or_default("GOCACHE", "/tmp/miniswar-go-build")
gomodcache := env_var_or_default("GOMODCACHE", "/tmp/miniswar-go-mod")

default:
    @just --list

test:
    GOCACHE={{gocache}} GOMODCACHE={{gomodcache}} go test ./...

build:
    GOCACHE={{gocache}} GOMODCACHE={{gomodcache}} go build -buildvcs=false ./...

check: test build

run:
    GOCACHE={{gocache}} GOMODCACHE={{gomodcache}} go run -buildvcs=false ./cmd/miniswar -addr {{addr}} -db {{db}}

run-local port:
    GOCACHE={{gocache}} GOMODCACHE={{gomodcache}} go run -buildvcs=false ./cmd/miniswar -addr 10.0.10.23:{{port}} -db /tmp/miniswar-{{port}}.sqlite

run-port port:
    GOCACHE={{gocache}} GOMODCACHE={{gomodcache}} go run -buildvcs=false ./cmd/miniswar -addr 127.0.0.1:{{port}} -db /tmp/miniswar-{{port}}.sqlite

fmt:
    gofmt -w cmd internal

clean:
    rm -f miniswar.sqlite /tmp/miniswar-*.sqlite
    rm -rf /tmp/miniswar-go-build
