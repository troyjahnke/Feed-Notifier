package main

import (
	"context"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/mmcdole/gofeed"
)

type Feed struct {
	Name   string `dynamodbav:"name"`
	Url    string `dynamodbav:"url"`
	Latest string `dynamodbav:"latest"`
}

func HandleRequest(ctx context.Context) {
	cfg, _ := config.LoadDefaultConfig(ctx, func(options *config.LoadOptions) error {
		options.Region = "us-east-1"
		return nil
	})
	svc := dynamodb.NewFromConfig(cfg)
	scannedFeeds, err := svc.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String("feeds"),
	})
	if err != nil {
		log.Fatalln(err.Error())
	}

	var feeds []Feed

	err = attributevalue.UnmarshalListOfMaps(scannedFeeds.Items, &feeds)
	if err != nil {
		log.Fatalln(err.Error())
	}

	fp := gofeed.NewParser()

	for _, feed := range feeds {
		parsedFeed, err := fp.ParseURL(feed.Url)
		if err != nil {
			log.Fatalln(err.Error())
		}
		latestLink := parsedFeed.Items[0].Link
		if latestLink != feed.Latest {
			update := expression.Set(expression.Name("latest"), expression.Value(latestLink))
			expr, err := expression.NewBuilder().WithUpdate(update).Build()
			if err != nil{
				log.Fatalln(err.Error())
			}
			_, err = svc.UpdateItem(ctx, &dynamodb.UpdateItemInput{
				TableName: aws.String("feeds"),
				Key: map[string]types.AttributeValue{
					"name": &types.AttributeValueMemberS{Value: feed.Name},
				},
				UpdateExpression: expr.Update(),
				ExpressionAttributeNames: expr.Names(),
				ExpressionAttributeValues: expr.Values(),

			})
			if err != nil {
				log.Fatalln(err.Error())
			}
		}
	}
}

func main() {
	lambda.Start(HandleRequest)
}
