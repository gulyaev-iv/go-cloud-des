# syntax=docker/dockerfile:1

FROM golang:1.26.2-bookworm AS build

WORKDIR /src

COPY api ./api
COPY controlplane ./controlplane

WORKDIR /src/controlplane

RUN go mod edit -replace github.com/gulyaev-iv/go-cloud-des/api=/src/api
RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/controlplane .

FROM alpine:3.21

RUN apk add --no-cache ca-certificates netcat-openbsd

COPY --from=build /out/controlplane /usr/local/bin/controlplane

EXPOSE 8080

ENTRYPOINT ["controlplane"]