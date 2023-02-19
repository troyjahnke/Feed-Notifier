ARG GO_VERSION
FROM public.ecr.aws/docker/library/golang:${GO_VERSION} AS builder

ADD *.go go.mod go.sum /app/
WORKDIR /app
RUN go build

FROM public.ecr.aws/lambda/go:latest
COPY --from=builder /app/FeedNotifier ${LAMBDA_TASK_ROOT}
CMD ["FeedNotifier"]