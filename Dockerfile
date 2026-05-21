FROM golang:1.25.7-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /app/bin/app ./main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/bin/app .
COPY migrations ./migrations

EXPOSE 8080
CMD ["./app"]