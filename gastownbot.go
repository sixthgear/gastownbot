// This file contains the main gastownbot event loop
// TODO: provide daily summary
// TODO: send reminders

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/nlopes/slack"
	"google.golang.org/api/calendar/v3"
)

type GastownBot struct {
	config        *GastownBotConfig
	slackRTM      *slack.RTM
	me            *slack.UserDetails
	defaultParams slack.PostMessageParameters
	gcal          *calendar.Service
	calSyncToken  string
	bookings      map[string]*Booking
	bookingsList  []*Booking
	next          *Booking
	timezone      *time.Location
	nextDaily     time.Time
}

type GastownBotConfig struct {
	SlackAPIToken   string
	SlackSlashToken string
	CalendarId      string
	Timezone        string
}

// Create and configure a new bot instance
func New(config *GastownBotConfig) (bot *GastownBot) {

	var err error
	bot = &GastownBot{config: config}

	// create slack API service
	api := slack.New(config.SlackAPIToken)
	bot.slackRTM = api.NewRTM()
	bot.defaultParams = slack.PostMessageParameters{AsUser: true}

	// create google calendar API service
	bot.gcal, err = NewService()
	if err != nil {
		log.Fatalf("Could not initialize google calendar service! %v", err)
	}

	// load timezone
	if bot.timezone, err = time.LoadLocation(config.Timezone); err != nil {
		log.Fatalf("Unabled to load timezone %s.", config.Timezone)
	}

	// set next daily update for 8am tomorrow
	tomorrow := time.Now() // .AddDate(0, 0, 1)
	bot.nextDaily = time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 8, 0, 0, 0, bot.timezone)
	fmt.Printf("Next daily update scheduled for %s\n", bot.nextDaily)

	// initialize bookings map
	bot.bookings = make(map[string]*Booking)

	return bot
}

// Run the bot.
func (bot *GastownBot) Go() bool {

	// channels for ticking
	var tick <-chan time.Time

	// first get all the bookings (no syncToken)
	if err := bot.SyncBookings(); err != nil {
		log.Fatalf(err.Error())
	}

	// now spin up RTM goroutine
	go bot.slackRTM.ManageConnection()

	// create http slash command handler
	http.HandleFunc("/slash", bot.HandleSlash)
	go http.ListenAndServe(":4000", nil)

	// infinite loop
	for {
		select {

		case t := <-tick:
			bot.Tick(t)

		case msg := <-bot.slackRTM.IncomingEvents:

			switch ev := msg.Data.(type) {

			case *slack.HelloEvent:
				fmt.Println("Hello.")

			case *slack.DisconnectedEvent:
				fmt.Println("Disconnected.")
				tick = nil

			case *slack.ConnectedEvent:
				fmt.Println("Connected.")
				bot.me = ev.Info.User
				tick = time.Tick(5 * time.Second)

			case *slack.MessageEvent:
				// ignore special message subtypes and my own messages
				if ev.SubType != "" || ev.User == bot.me.ID {
					continue
				}

				// only proceed on mentions
				// if !strings.Contains(ev.Text, bot.me.Name) {
				// 	continue
				// }

				// userInfo, err := bot.slackRTM.GetUserInfo(ev.User)
				// if err != nil {
				// 	fmt.Println("Unable to fetch user info.")
				// 	continue
				// }

				// parse and run command
				// command, args := bot.Parse(ev.Text)
				// command(ev.Channel, userInfo.Name, args)

			case *slack.PresenceChangeEvent:
				fmt.Printf("Presence Change: %v\n", ev)

			case *slack.LatencyReport:
				fmt.Printf("Current latency: %v\n", ev.Value)

			case *slack.RTMError:
				fmt.Printf("Error: %s\n", ev.Error())

			case *slack.InvalidAuthEvent:
				return false

			default:
				// Ignore other events..
			}
		}
	}

	return true
}

// Slash command handler
func (bot *GastownBot) HandleSlash(w http.ResponseWriter, req *http.Request) {

	var response []byte

	if req.PostFormValue("token") != bot.config.SlackSlashToken {
		w.WriteHeader(403)
		io.WriteString(w, "Not authenticated.")
		return
	}

	channel := req.PostFormValue("channel_id")
	username := req.PostFormValue("user_name")
	text := req.PostFormValue("text")

	tokens := strings.Split(strings.TrimSpace(text), " ")
	for _, t := range tokens {
		t = strings.TrimSpace(strings.ToLower(t))
	}

	fmt.Println(tokens)

	switch {
	case len(tokens) == 0, tokens[0] == "":
		response, _ = json.Marshal(bot.Help(channel, username))

	case tokens[0] == "help":
		response, _ = json.Marshal(bot.Help(channel, username))

	case tokens[0] == "show":
		response, _ = json.Marshal(bot.Show(channel, username))

	default:
		response, _ = json.Marshal(bot.Book(channel, username, text))
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(response)

}

// Tick function executes periodically
func (bot *GastownBot) Tick(now time.Time) {

	// sync with google calendar
	if err := bot.SyncBookings(); err != nil {
		log.Printf(err.Error())
		return
	}

	// process daily tasks
	if now.After(bot.nextDaily) {
		bot.nextDaily = bot.nextDaily.Add(24 * time.Hour)
		msg := bot.Show("", "")
		params := bot.defaultParams
		params.Attachments = msg.Attachments
		bot.Broadcast("", msg.Text, &params)
	}
}

// Braodcast a message to all connected channels
func (bot *GastownBot) Broadcast(channel string, text string, params *slack.PostMessageParameters) {

	// Post to a specific channel
	if channel != "" {
		bot.slackRTM.PostMessage(channel, text, *params)
		return
	}

	// Post to all connected groups and channels
	groups, _ := bot.slackRTM.GetGroups(true)
	channels, _ := bot.slackRTM.GetChannels(true)
	for _, group := range groups {
		bot.slackRTM.PostMessage(group.ID, text, *params)
	}
	for _, channel := range channels {
		bot.slackRTM.PostMessage(channel.ID, text, *params)
	}
}

// Set the topic for all channels
func (bot *GastownBot) Topic(topic string) {
	groups, _ := bot.slackRTM.GetGroups(true)
	channels, _ := bot.slackRTM.GetChannels(true)
	for _, group := range groups {
		if group.Topic.Value != topic {
			bot.slackRTM.SetGroupTopic(group.ID, topic)
		}
	}
	for _, channel := range channels {
		if channel.Topic.Value != topic {
			bot.slackRTM.SetChannelTopic(channel.ID, topic)
		}
	}
}

// Set the topic to show the next booking (if needed)
func (bot *GastownBot) SetNextTopic() bool {
	var topic string
	if bot.next != nil {
		topic = fmt.Sprintf("*Next Booking:* %s %s", bot.next.TimeString(), bot.next.What)
	} else {
		topic = "No bookings today."
	}
	bot.Topic(topic)
	return true
}

// Grab the bookings for the day and send them to channels
func (bot *GastownBot) RemindNext(args []string) bool {

	if bot.next == nil {
		return false
	}
	params := bot.defaultParams
	attachments := []slack.Attachment{}

	attachments = append(attachments, slack.Attachment{
		Color: "#3BCBFF",
		Fields: []slack.AttachmentField{
			slack.AttachmentField{Title: "Time", Value: bot.next.TimeString(), Short: true},
			slack.AttachmentField{Title: "Person", Value: bot.next.What, Short: true},
		},
	})

	params.Attachments = attachments
	bot.Broadcast("", "*Reminder*", &params)
	return true
}
