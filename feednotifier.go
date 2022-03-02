package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/containrrr/shoutrrr"
	"github.com/go-redis/redis/v8"
	"github.com/jasonlvhit/gocron"
	"github.com/mmcdole/gofeed"
	"log"
	"os"
	"strconv"
	"time"
)

var ctx = context.Background()
var db = createRedisClient()

type Feed struct {
	Name string `json:"name"`
	Url  string `json:"url"`
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
	timeoutstr, t := os.LookupEnv("TIMEOUT")
	if !t {
		timeoutstr = "10800"
	}
	delaystr, t := os.LookupEnv("DELAY")
	if !t {
		delaystr = "5"
	}
	notificationurl, n := os.LookupEnv("NOTIFICATION_URL")
	if !n {
		log.Fatalln("Timeout and/or notification url is missing.")
	}
	err := shoutrrr.Send(notificationurl, "Feed Notifier Started...")
	if err != nil {
		log.Fatalln("Failed to send test message: " + err.Error())
	}
	feedpath, fe := os.LookupEnv("FEED_FILE_PATH")
	if !fe {
		feedpath = "/feeds.json"
	}
	feedfile, err := os.ReadFile(feedpath)
	if err != nil {
		log.Fatalln("Failed to read feed json file. " + err.Error())
	}
	timeout, timeouterr := strconv.ParseUint(timeoutstr, 10, 64)
	delay, delayerr := strconv.ParseInt(delaystr, 10, 64)
	if timeouterr != nil || delayerr != nil {
		log.Fatalln("Unable to convert timeout to integer.")
	}

	var feeds []Feed
	json.Unmarshal(feedfile, &feeds)
	plan := gocron.NewScheduler()

	for _, f := range feeds {
		task(f.Name, f.Url, notificationurl)
		time.Sleep(time.Duration(delay) * time.Second)
		plan.Every(timeout).Seconds().Do(task, f.Name, f.Url, notificationurl)
	}
	<-plan.Start()
}
