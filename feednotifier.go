package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/containrrr/shoutrrr"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/mmcdole/gofeed"
)

type Feed struct {
	Name   string `dynamodbav:"name"`
	Url    string `dynamodbav:"url"`
	Latest string `dynamodbav:"latest"`
}

type ShoutrrrSecret struct {
	Url string `json:"shoutrrrUrl"`
}

func HandleRequest(ctx context.Context) {
	// Initialize services.
	cfg, _ := config.LoadDefaultConfig(ctx, func(options *config.LoadOptions) error {
		options.Region = "us-east-1"
		return nil
	})
	dynamodbService := dynamodb.NewFromConfig(cfg)
	secretManager := secretsmanager.NewFromConfig(cfg)

	// Setup notification URL.
	shoutrrrUrl, err := secretManager.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String("notificationFinal"),
	})
	if err != nil {
		log.Fatalln("Failed to get notification URL: " + err.Error())
	}

	// Scan for feeds.
	scannedFeeds, err := dynamodbService.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String("feeds"),
	})
	if err != nil {
		log.Fatalln("Failed to get feeds: " + err.Error())
	}
	var feeds []Feed
	err = attributevalue.UnmarshalListOfMaps(scannedFeeds.Items, &feeds)
	if err != nil {
		log.Fatalln("Failed to parse feeds: " + err.Error())
	}

	fp := gofeed.NewParser()

	for _, feed := range feeds {
		parsedFeed, err := fp.ParseURL(feed.Url)
		if err != nil {
			log.Fatalln("Failed to parse the feed URL: " + err.Error())
		}
		latestFeed := parsedFeed.Items[0]
		latestLink := latestFeed.Link
		log.Printf("Stored Info: %s | %s | %s -> %s", feed.Name, feed.Url, feed.Latest, latestLink)
		if latestLink != feed.Latest {
			update := expression.Set(expression.Name("latest"), expression.Value(latestLink))
			expr, err := expression.NewBuilder().WithUpdate(update).Build()
			if err != nil {
				log.Fatalln("Failed to build query expression: " + err.Error())
			}
			_, err = dynamodbService.UpdateItem(ctx, &dynamodb.UpdateItemInput{
				TableName: aws.String("feeds"),
				Key: map[string]types.AttributeValue{
					"name": &types.AttributeValueMemberS{Value: feed.Name},
				},
				UpdateExpression:          expr.Update(),
				ExpressionAttributeNames:  expr.Names(),
				ExpressionAttributeValues: expr.Values(),
			})
			if err != nil {
				log.Fatalln("Failed to update entry: " + err.Error())
			}
			var shoutrrrEntry ShoutrrrSecret
			if json.Unmarshal([]byte(*shoutrrrUrl.SecretString), &shoutrrrEntry) != nil {
				log.Fatalln("Failed to parse notification URL")
			}
			if err = shoutrrr.Send(shoutrrrEntry.Url,
				fmt.Sprintf("%s - %s - %s", feed.Name, latestFeed.Title, latestLink)); err != nil {
				log.Fatalln("Failed to send notification: " + err.Error())
			}
		}
	}
}

func main() {
	lambda.Start(HandleRequest)
}
