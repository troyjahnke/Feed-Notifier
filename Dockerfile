ARG GO_VERSION
FROM golang:${GO_VERSION}
WORKDIR /app

ADD feednotifier.go ./
ADD go.mod ./
ADD go.sum ./


RUN go build
CMD ./FeedNotifier