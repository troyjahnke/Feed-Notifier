ARG GO_VERSION=1.18.3
FROM golang:${GO_VERSION}-alpine as builder
WORKDIR /app
ADD feednotifier.go go.mod go.sum ./
RUN go build

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/FeedNotifier ./
CMD ./FeedNotifier