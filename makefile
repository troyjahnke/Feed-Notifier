update:
	cd src
	go mod tidy
	go get -u
taint:
	terraform taint null_resource.feed_notifier
	terraform taint aws_lambda_function.feed_notifier
deploy: taint
	terraform apply
clean:
	cd src
	rm FeedNotifier FeedNotifier.zip
test:
	aws lambda invoke --function-name FeedNotifier test.json
	cat test.json
	rm test.json
all: clean update deploy