FROM golang:1.21 AS builder

ENV SERVICE_TAG=example

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN curl -o default.pgo http://host.docker.internal:8080/profile/${SERVICE_TAG} || true
RUN GOOS=linux GOARCH=amd64 go build -o main ./examples/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/main .

EXPOSE 8080

CMD ["./main"]
