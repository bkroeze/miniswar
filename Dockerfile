FROM golang:1.26-bookworm AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -buildvcs=false -trimpath=false -o /out/miniswar ./cmd/miniswar

FROM debian:bookworm-slim

WORKDIR /app

RUN useradd --create-home --uid 10001 miniswar \
	&& mkdir -p /data \
	&& chown -R miniswar:miniswar /app /data

COPY --from=build /out/miniswar /app/miniswar
COPY --from=build --chown=miniswar:miniswar /app/data /app/data
COPY --from=build --chown=miniswar:miniswar /app/web /app/web

USER miniswar

EXPOSE 8080
VOLUME ["/data"]

ENTRYPOINT ["/app/miniswar"]
CMD ["-addr", "0.0.0.0:8080", "-db", "/data/miniswar.sqlite"]
