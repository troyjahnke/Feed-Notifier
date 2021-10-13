package main

import (
	"crypto/sha1"
	"fmt"
	"github.com/jasonlvhit/gocron"
	"github.com/mmcdole/gofeed"
	tb "gopkg.in/tucnak/telebot.v2"
	"log"
	"os"
	"strconv"
	"strings"
)

var cache map[string][20]byte

func task(name string, url string, bot *tb.Bot, user *tb.User) {
	parser := gofeed.NewParser()
	feed, _ := parser.ParseURL(url)
	newcache := sha1.Sum([]byte(feed.Items[0].Link))
	cacheentry := cache[name]
	if &cacheentry != nil {
		if cacheentry != newcache {
			bot.Send(user, fmt.Sprintf("%s - %s", name, feed.Items[0].Title))
		}
	} else {
		bot.Send(user, fmt.Sprintf("%s - %s", name, feed.Items[0].Title))
	}
	cache[name] = newcache
}

func main() {
	cache = map[string][20]byte{}

	telegramtoken, tt := os.LookupEnv("TELEGRAM_TOKEN")
	telegramchannelstr, tc := os.LookupEnv("TELEGRAM_ID")
	feedsstr, f := os.LookupEnv("FEEDS")
	timeoutstr, t := os.LookupEnv("TIMEOUT")
	if !tt || !tc || !f || !t {
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

	feeds := strings.Split(feedsstr, ",")

	for _, f := range feeds {
		feedentry := strings.Split(f, "|")
		task(feedentry[0], feedentry[1], bot, &user)
		plan.Every(uint64(timeout)).Seconds().Do(task, feedentry[0], feedentry[1], bot, &user)
	}
	<-plan.Start()
}
