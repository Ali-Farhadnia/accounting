# Build downloads modules (vendor/ is not in git; use go mod vendor locally for air-gapped builds).
FROM golang:1.22-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN go build -trimpath -ldflags="-s -w" -o /accounting .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata \
	&& adduser -D -H -s /sbin/nologin -u 65532 app

USER 65532:65532
WORKDIR /data

COPY --from=build /accounting /usr/local/bin/accounting

EXPOSE 8080
VOLUME ["/data"]

ENV ACCOUNTING_HTTP_ADDR=:8080 \
	ACCOUNTING_DB=/data/app.db \
	ACCOUNTING_TZ=Asia/Tehran \
	ACCOUNTING_LANG=fa \
	ACCOUNTING_PASSWORD="" \
	ACCOUNTING_AUTH_KEY=""

ENTRYPOINT ["/usr/local/bin/accounting"]
