FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.22

RUN addgroup -S app && adduser -S app -G app && apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/server /app/server
RUN mkdir -p /app/data && chown -R app:app /app

USER app
ENV HTTP_ADDR=:8080
ENV DATA_FILE=/app/data/app_state.json
EXPOSE 8080

ENTRYPOINT ["/app/server"]
