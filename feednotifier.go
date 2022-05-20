package main

import (
	"context"
	"fmt"
	"github.com/containrrr/shoutrrr"
	"github.com/go-redis/redis/v8"
	"github.com/jasonlvhit/gocron"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/mmcdole/gofeed"
	"log"
	"os"
	"strings"
	"time"
)

var ctx = context.Background()
var db = createRedisClient()
var conf = koanf.New(".")

type Feed struct {
	Name string
	Url  string
}

func task(name string, url string, notificationurl string) {
	log.Println(fmt.Sprintf("Looking for updates for %s - %s", name, url))
	parser := gofeed.NewParser()
	feed, _ := parser.ParseURL(url)
	newcache := feed.Items[0].Link
	cacheentry, err := db.Get(ctx, name).Result()

	if err == redis.Nil || newcache != cacheentry {
		shoutrrr.Send(notificationurl, fmt.Sprintf("%s - %s - %s", name, feed.Items[0].Title, feed.Items[0].Link))
	}
	err = db.Set(ctx, name, newcache, 0).Err()
	if err != nil {
		log.Println("Failed to store entry in redis cache.")
	}
}

func createRedisClient() *redis.Client {
	redisaddr, v := os.LookupEnv("REDIS_ADDR")
	if !v {
		redisaddr = "redis:6379"
	}
	redispass, v := os.LookupEnv("REDIS_PASS")
	if !v {
		redispass = ""
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisaddr,
		Password: redispass, // no password set
		DB:       0,         // use default DB
	})
	return rdb
}

func main() {
	log.SetOutput(os.Stdout)
	log.Println("Starting Feed notifier...")
	conf.Load(confmap.Provider(map[string]interface{}{
		"timeout": 10800,
		"delay":   5,
	}, "."), nil)
	conf.Load(file.Provider("/config.json"), json.Parser())
	conf.Load(file.Provider("./config.json"), json.Parser())
	conf.Load(env.Provider("", ".", func(s string) string {
		return strings.ToLower(s)
	}), nil)

	// Setup notifier
	notification_url := conf.MustString("notification_url")
	err := shoutrrr.Send(notification_url, "Feed Notifier Started...")
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
	timeout := uint64(conf.MustInt64("timeout"))
	delay := uint64(conf.MustInt64("delay"))
	plan := gocron.NewScheduler()

	// Run notifier.
	for _, f := range feeds {
		task(f.Name, f.Url, notification_url)
		time.Sleep(time.Duration(delay) * time.Second)
		plan.Every(timeout).Seconds().Do(task, f.Name, f.Url, notification_url)
	}
	<-plan.Start()
}
