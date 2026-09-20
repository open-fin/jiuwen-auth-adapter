FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/jiuwen-auth-adapter ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/jiuwen-auth-adapter /usr/local/bin/jiuwen-auth-adapter
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/jiuwen-auth-adapter"]
