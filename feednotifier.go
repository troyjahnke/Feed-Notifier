package main

import (
	"context"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func HandleRequest(ctx context.Context) {
	print("hello world")
	cfg, _ := config.LoadDefaultConfig(ctx, func(options *config.LoadOptions) error {
		return nil
	})
	svc := dynamodb.NewFromConfig(cfg)
	feeds, err := svc.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String("Feeds"),
	})
	if err != nil {
		panic(err)
	}

	for feed := range feeds.Items {
		print(feed)
	}
}

func main() {
	lambda.Start(HandleRequest)
}
