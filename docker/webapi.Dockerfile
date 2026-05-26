# syntax=docker/dockerfile:1

FROM golang:1.26.2-bookworm AS build

WORKDIR /src

COPY api ./api
COPY webapi ./webapi

WORKDIR /src/webapi

RUN go mod edit -replace github.com/gulyaev-iv/go-cloud-des/api=/src/api
RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/webapi .

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

COPY --from=build /out/webapi /usr/local/bin/webapi

EXPOSE 8081

ENTRYPOINT ["webapi"]