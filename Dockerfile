FROM golang:1.25.12-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY api ./api
COPY cmd ./cmd
COPY internal ./internal
COPY stepup ./stepup
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/identity ./cmd/identity

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=build /out/identity /usr/local/bin/identity
COPY manifest ./manifest

ENV OTEL_TRACES_EXPORTER=none

EXPOSE 8081
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/identity"]
