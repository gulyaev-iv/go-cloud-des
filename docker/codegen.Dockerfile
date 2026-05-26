# syntax=docker/dockerfile:1

FROM golang:1.26.2-bookworm AS build

WORKDIR /src

COPY codegen ./codegen

WORKDIR /src/codegen

RUN go mod download
RUN go build -o /out/codegen ./cmd/codegen

FROM golang:1.26.2-bookworm

WORKDIR /app/codegen

COPY --from=build /go/pkg/mod /go/pkg/mod
COPY --from=build /root/.cache/go-build /root/.cache/go-build
COPY --from=build /src/codegen /app/codegen
COPY --from=build /out/codegen /usr/local/bin/codegen

RUN mkdir -p /data/codegen-work

ENTRYPOINT ["codegen"]