ARG GO_VERSION
FROM golang:${GO_VERSION}
WORKDIR /app

ADD feednotifier.go o.mod go.sum ./

RUN go build
CMD ./FeedNotifier