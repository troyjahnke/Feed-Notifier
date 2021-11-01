package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/jasonlvhit/gocron"
	"github.com/mmcdole/gofeed"
	tb "gopkg.in/tucnak/telebot.v2"
	"log"
	"os"
	"strconv"
)

var ctx = context.Background()
var db = createRedisClient()

type Feed struct {
	Name string `json:"name"`
	Url string `json:"url"`
}

func task(name string, url string, bot *tb.Bot, user *tb.User) {
	parser := gofeed.NewParser()
	feed, _ := parser.ParseURL(url)
	newcache := feed.Items[0].Link
	cacheentry, err := db.Get(ctx, name).Result()
	if err == redis.Nil {
		bot.Send(user, fmt.Sprintf("%s - %s - %s", name, feed.Items[0].Title, feed.Items[0].Link))
	} else {
		if newcache != cacheentry {
			bot.Send(user, fmt.Sprintf("%s - %s - %s", name, feed.Items[0].Title, feed.Items[0].Link))
		}
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
		DB:       0,  // use default DB
	})
	return rdb
}

func main() {
	telegramtoken, tt := os.LookupEnv("TELEGRAM_TOKEN")
	telegramchannelstr, tc := os.LookupEnv("TELEGRAM_ID")
	timeoutstr, t := os.LookupEnv("TIMEOUT")
	feedpath, fe := os.LookupEnv("FEED_FILE_PATH")
	if !fe{
		feedpath = "/feeds.json"
	}
	feedfile, err := os.ReadFile(feedpath)
	if err != nil{
		log.Fatalln("Failed to read feed json file. " + err.Error())
	}
	var feeds []Feed
	json.Unmarshal(feedfile, &feeds)
	if !tt || !tc || !t {
		log.Fatalln("Telegram Token, Telegram Channel ID, feeds, and timeout are required")
	}

	telegramchannel, err := strconv.Atoi(telegramchannelstr)
	if err != nil {
		log.Fatalln("Unable to convert telegram channel to integer.")
	}

	timeout, err := strconv.Atoi(timeoutstr)
	if err != nil {
		log.Fatalln("Unable to convert timeout to integer.")
	}

	bot, err := tb.NewBot(tb.Settings{Token: telegramtoken})
	if err != nil {
		log.Fatalln("Unable to create telegram bot.")
	}

	user := tb.User{ID: telegramchannel}

	plan := gocron.NewScheduler()

	for _, f := range feeds {
		task(f.Name, f.Url, bot, &user)
		plan.Every(uint64(timeout)).Seconds().Do(task, f.Name, f.Url, bot, &user)
	}
	<-plan.Start()
}
