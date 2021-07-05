FROM golang:1.15.8-alpine as builder
WORKDIR /app/
COPY . /app/
RUN go build -o app/bin/pg-api ./cmd/pg-api

FROM busybox:glibc as app
COPY --from=builder /app/pg-api /app/pg-api
COPY --from=builder /app/config/dummy.json /app/dummy.json
RUN chmod +x /app-/pg-api
CMD ["/app/pg-api", "/app/dummy.json"]

