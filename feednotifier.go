package main

import (
	"fmt"
	"github.com/containrrr/shoutrrr"
	"github.com/go-co-op/gocron"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/mmcdole/gofeed"
	"github.com/ostafen/clover"
	"log"
	"os"
	"strings"
	"time"
)

var conf = koanf.New(".")

type Feed struct {
	Name string
	Url  string
}

func task(name string, url string, notificationUrl string, db *clover.DB) {
	log.Println(fmt.Sprintf("Looking for updates for %s - %s", name, url))
	parser := gofeed.NewParser()
	feed, _ := parser.ParseURL(url)
	latestLink := feed.Items[0].Link

	existingDoc, _ := db.Query("feeds").Where(clover.Field("name").Eq(name)).FindFirst()

	var err error
	if existingDoc == nil || existingDoc.Get("url").(string) != latestLink {
		err = shoutrrr.Send(notificationUrl,
			fmt.Sprintf("%s - %s - %s", name, feed.Items[0].Title, latestLink))
	}
	if existingDoc != nil {
		existingDoc.Set("url", latestLink)
		err = db.Save("feeds", existingDoc)
	} else {
		existingDoc = clover.NewDocument()
		existingDoc.Set("name", name)
		existingDoc.Set("url", latestLink)
		_, err = db.InsertOne("feeds", existingDoc)
	}
	if err != nil {
		log.Fatalln(err.Error())
	}
}

func main() {
	log.SetOutput(os.Stdout)
	log.Println("Starting Feed notifier...")

	conf.Load(confmap.Provider(map[string]interface{}{
		"timeout":            10800,
		"delay":              5,
		"db_collection_name": "feeds",
		"db_path":            "/config/db",
	}, "."), nil)
	conf.Load(file.Provider("/config.json"), json.Parser())
	conf.Load(file.Provider("./config.json"), json.Parser())
	conf.Load(env.Provider("", ".", func(s string) string {
		return strings.ToLower(s)
	}), nil)

	db, err := clover.Open(conf.MustString("db_path"))
	if err != nil {
		log.Fatalln("Failed to open DB. " + err.Error())
	}
	collectionExists, err := db.HasCollection("feeds")
	if err == nil && !collectionExists {
		err = db.CreateCollection("feeds")
	}
	if err != nil {
		log.Fatalln("Failed to create feed collection.")
	}

	// Setup notifier
	notificationUrl := conf.MustString("notification_url")

	err = shoutrrr.Send(notificationUrl, "Feed Notifier Started...")
	if err != nil {
		log.Fatalln("Failed to send test message: " + err.Error())
	}

	// Setup feeds and feed scheduler
	var feeds []Feed
	conf.Unmarshal("feeds", &feeds)

	feedlength := len(feeds)
	if feedlength == 0 {
		log.Fatalln("No feeds defined. Exiting now.")
	} else {
		log.Printf("Detected %d feeds", feedlength)
	}

	// Setup scheduler.
	timeout := conf.MustInt("timeout")
	plan := gocron.NewScheduler(time.UTC)

	// Run notifier.
	for _, f := range feeds {
		_, err := plan.Every(timeout).Seconds().Do(task, f.Name, f.Url, notificationUrl, db)
		if err != nil {
			log.Fatalf("Failed to create task for %s\nError: %s", f.Name, err.Error())
		}
	}
	plan.StartBlocking()
}
