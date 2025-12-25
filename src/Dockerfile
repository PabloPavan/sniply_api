# syntax=docker/dockerfile:1

FROM golang:alpine AS build
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api ./cmd/api

FROM alpine:latest
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /bin/api /app/api

EXPOSE 8080
CMD ["/app/api"]
