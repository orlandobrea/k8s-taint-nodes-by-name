FROM golang:1.23 AS builder
WORKDIR /app
COPY ./src .
RUN GOOS=linux GOARCH=amd64 go build -o node-taint-controller

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/node-taint-controller .
ENTRYPOINT ["/app/node-taint-controller"]
