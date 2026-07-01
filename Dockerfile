FROM golang:1.26-bookworm AS build

LABEL service=miniswar

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG APP_VERSION
ARG APP_BRANCH
ARG APP_DEFAULT_BRANCH=main
RUN apt-get update \
   && apt-get install -y --no-install-recommends curl \
   && rm -rf /var/lib/apt/lists/*
RUN APP_VERSION="${APP_VERSION:-$(cat internal/version/VERSION)}" \
	&& CGO_ENABLED=0 go build -buildvcs=false -trimpath=false \
		-ldflags "-X 'miniswar/internal/version.baseVersion=${APP_VERSION}' -X 'miniswar/internal/version.branchName=${APP_BRANCH}' -X 'miniswar/internal/version.defaultBranch=${APP_DEFAULT_BRANCH}'" \
		-o /out/miniswar ./cmd/miniswar

FROM debian:bookworm-slim

WORKDIR /app

RUN useradd --create-home --uid 10001 miniswar \
	&& mkdir -p /storage \
	&& chown -R miniswar:miniswar /app /storage

COPY --from=build /out/miniswar /app/miniswar
COPY --from=build --chown=miniswar:miniswar /app/data /app/data
COPY --from=build --chown=miniswar:miniswar /app/web /app/web

USER miniswar

EXPOSE 8080
VOLUME ["/storage"]

ENTRYPOINT ["/app/miniswar"]
CMD ["-addr", "0.0.0.0:8080", "-db", "/storage/miniswar.sqlite"]
