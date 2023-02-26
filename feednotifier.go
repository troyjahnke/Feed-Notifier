package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"log"
	"os"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/containrrr/shoutrrr"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/mmcdole/gofeed"
)

type Feed struct {
	Name    string `dynamodbav:"name"`
	Url     string `dynamodbav:"url"`
	Latest  string `dynamodbav:"latest"`
	Pattern string `dynamodbav:"pattern"`
}

type FeedInfo interface {
	GetFeedInfo() (string, []Feed)
	UpdateFeedInfo(feedName string, latestLink string) error
}

type AwsInfo struct {
	DBClient      *dynamodb.Client
	SecretManager *secretsmanager.Client
	Ctx           context.Context
}

type JsonInfo struct{}

func (jsonInfo JsonInfo) GetFeedInfo() (string, []Feed) {
	shoutrrrUrl := os.Getenv("NOTIFICATION_URL")
	feedBytes, err := os.ReadFile("feeds.json")
	if err != nil {
		log.Fatalln(err)
	}
	var feeds []Feed
	err = json.Unmarshal(feedBytes, &feeds)
	if err != nil {
		log.Fatalln(err)
	}
	notificationUrl, err := json.Marshal(shoutrrrUrl)
	if err != nil {
		log.Fatalln(err)
	}
	return string(notificationUrl), feeds
}

func (jsonInfo JsonInfo) UpdateFeedInfo(feedName string, latestLink string) error {
	feedBytes, err := os.ReadFile("feeds.json")
	if err != nil {
		log.Fatalln(err)
	}
	var feeds []Feed
	err = json.Unmarshal(feedBytes, &feeds)
	if err != nil {
		log.Fatalln(err)
	}
	for _, feed := range feeds {
		if feed.Name == feedName {
			feed.Latest = latestLink
		}
	}
	feedBytes, err = json.Marshal(feeds)
	err = os.WriteFile("feeds.json", feedBytes, 0644)
	return err
}

func (awsInfo AwsInfo) GetFeedInfo() (string, []Feed) {
	// Setup notification URL.
	shoutrrrUrl, err := awsInfo.SecretManager.GetSecretValue(awsInfo.Ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(os.Getenv("SECRET_NAME")),
	})
	if err != nil {
		log.Fatalln("Failed to get notification URL: " + err.Error())
	}

	// Scan for feeds.
	scannedFeeds, err := awsInfo.DBClient.Scan(awsInfo.Ctx, &dynamodb.ScanInput{
		TableName: aws.String(os.Getenv("TABLE_NAME")),
	})
	if err != nil {
		log.Fatalln("Failed to get feeds: " + err.Error())
	}
	var feeds []Feed
	err = attributevalue.UnmarshalListOfMaps(scannedFeeds.Items, &feeds)
	if err != nil {
		log.Fatalln("Failed to parse feeds: " + err.Error())
	}
	return *shoutrrrUrl.SecretString, feeds
}

func (awsInfo AwsInfo) UpdateFeedInfo(feedName string, latestLink string) error {
	update := expression.Set(expression.Name("latest"), expression.Value(latestLink))
	expr, err := expression.NewBuilder().WithUpdate(update).Build()
	if err != nil {
		log.Fatalln("Failed to build query expression: " + err.Error())
	}
	_, err = awsInfo.DBClient.UpdateItem(awsInfo.Ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String("feeds"),
		Key: map[string]types.AttributeValue{
			"name": &types.AttributeValueMemberS{Value: feedName},
		},
		UpdateExpression:          expr.Update(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	})
	return err
}

func HandleRequest(ctx context.Context) {
	_, isAws := os.LookupEnv("AWS_LAMBDA_FUNCTION_NAME")
	var feedInfo FeedInfo
	// Initialize services.
	cfg, _ := config.LoadDefaultConfig(ctx, func(options *config.LoadOptions) error {
		return nil
	})
	if isAws {
		dynamodbService := dynamodb.NewFromConfig(cfg)
		secretManager := secretsmanager.NewFromConfig(cfg)
		feedInfo = AwsInfo{
			SecretManager: secretManager,
			DBClient:      dynamodbService,
			Ctx:           ctx,
		}
	} else {
		feedInfo = JsonInfo{}
	}
	shoutrrrUrl, feeds := feedInfo.GetFeedInfo()
	// Construct the feed parser. This is used to perform the request and parse the items in
	// the syndication feed.
	fp := gofeed.NewParser()

	// Iterate over feed URLs in the list from AWS.
	for _, feed := range feeds {
		log.Printf("Processing %s | %s| %s", feed.Name, feed.Url, feed.Latest)
		parsedFeed, err := fp.ParseURL(feed.Url)
		if err != nil {
			log.Fatalln("Failed to parse the feed URL: " + err.Error())
		}
		log.Printf("Processing %+v", feed)

		var pattern *regexp.Regexp = nil
		if feed.Pattern != "" {
			pattern, err = regexp.Compile(feed.Pattern)
			if err != nil {
				log.Printf("Failed to create pattern for %s", feed.Pattern)
			}
		}
		matchFound := false
		// Iterate over items in the syndication feed.
		for _, feedItem := range parsedFeed.Items {
			feedLink := feedItem.Link
			if pattern != nil {
				if !pattern.MatchString(feedLink) {
					continue
				} else {
					// We found a match but we still need to check to see if this matches the latest link to see if its
					// an update.
					log.Printf("Pattern matched: %s", feedLink)
					matchFound = true
				}
			}
			if feedLink != feed.Latest {
				log.Printf("Updating %s: %s -> %s", feed.Name, feed.Latest, feedLink)
				err = feedInfo.UpdateFeedInfo(feed.Name, feedLink)
				if err != nil {
					log.Fatalln("Failed to update entry: " + err.Error())
				}
				if err = shoutrrr.Send(shoutrrrUrl,
					fmt.Sprintf("%s - %s - %s", feed.Name, feedItem.Title, feedLink)); err != nil {
					log.Fatalln("Failed to send notification: " + err.Error())
				}
				matchFound = true
			}
			if matchFound || pattern == nil {
				// If the pattern is nil, we just want to compare against the newest entry and stop.
				break
			}
		}
	}
}

func main() {
	_, isAws := os.LookupEnv("AWS_LAMBDA_FUNCTION_NAME")
	if isAws {
		lambda.Start(HandleRequest)
	} else {
		HandleRequest(context.TODO())
	}
}
