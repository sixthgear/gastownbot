package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/nlopes/slack"
	"google.golang.org/api/calendar/v3"
)

// Booking struct
type Booking struct {
	Bot        *GastownBot // reference to the parent bot
	Id         string      // ...
	CalendarId string      // ..
	What       string      // ..
	Start      time.Time   // ...
	End        time.Time   //
	// Deleted    bool
	// Reminded   bool      //

}

// Sorting type for bookings
type ByStartTime []*Booking

func (a ByStartTime) Len() int           { return len(a) }
func (a ByStartTime) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByStartTime) Less(i, j int) bool { return a[i].Start.Before(a[j].Start) }

// Fetch all the bookings.
func (bot *GastownBot) SyncBookings() error {
	// MaxResults(num).
	// OrderBy("startTime")
	// ShowDeleted(false).
	call := bot.gcal.Events.List(bot.config.CalendarId)

	if bot.calSyncToken != "" {
		// incremental
		call = call.SyncToken(bot.calSyncToken)
	} else {
		// full retrieve
		t := time.Now().Format(time.RFC3339)
		call = call.ShowDeleted(false).SingleEvents(true).TimeMin(t)
	}

	// execute the call
	// TODO: don't crash if the call fails
	events, err := call.Do()
	if err != nil {
		return fmt.Errorf("Unable to retrieve calendar events: %v", err)
	}

	// set next sync token
	bot.calSyncToken = events.NextSyncToken

	if len(events.Items) > 0 {

		// var bookings []*Booking
		var numChanged, numDeleted, numAdded int64

		for _, event := range events.Items {

			_, exists := bot.bookings[event.Id]
			booking := BookingFromEvent(bot, bot.config.CalendarId, event)

			if booking == nil && exists {
				numDeleted++
				delete(bot.bookings, event.Id)
			} else if booking == nil {
				numDeleted++
			} else if exists {
				numChanged++
				bot.bookings[booking.Id] = booking
			} else {
				numAdded++
				bot.bookings[booking.Id] = booking
			}

		}

		if numChanged > 0 {
			fmt.Printf("%d updated bookings.\n", numChanged)
		}
		if numDeleted > 0 {
			fmt.Printf("%d deleted bookings.\n", numDeleted)
		}
		if numAdded > 0 {
			fmt.Printf("%d added bookings.\n", numAdded)
		}

		// fmt.Printf("New bookings length: %d\n", len(bookings))
		fmt.Printf("New bookings map length: %d\n", len(bot.bookings))
	}

	// generate new bookings list, removing expired items
	bot.bookingsList = make([]*Booking, 0)
	for _, b := range bot.bookings {
		if b.End.After(time.Now()) {
			bot.bookingsList = append(bot.bookingsList, b)
		}
	}

	// sort new list by start time
	sort.Sort(ByStartTime(bot.bookingsList))
	// replace original list
	// bot.bookings = bookings

	// check if next booking has changed
	if len(bot.bookingsList) > 0 {
		bot.next = bot.bookingsList[0]
	} else {
		bot.next = nil
	}

	// set topic (if required)
	defer bot.SetNextTopic()

	// return events.NextSyncToken
	return nil
}

// Creates a booking object from an event object
func BookingFromEvent(bot *GastownBot, calendarID string, event *calendar.Event) *Booking {

	if event.Status == "cancelled" {
		return nil
	}

	b := new(Booking)
	b.Bot = bot
	b.CalendarId = calendarID
	b.Id = event.Id
	b.Start, _ = time.Parse(time.RFC3339, event.Start.DateTime)
	b.End, _ = time.Parse(time.RFC3339, event.End.DateTime)
	b.What = event.Summary
	// b.Deleted = event.Status == "cancelled"
	return b
}

// Checks if the start date is within a specified range
func (b *Booking) IsWithin(startTime time.Time, endTime time.Time) bool {
	return (b.Start.Equal(startTime) || b.Start.After(startTime)) && b.Start.Before(endTime)
}

// Returns a friendly time string
func (b *Booking) TimeString() string {

	dateString := ""
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, b.Bot.timezone)
	start := time.Date(b.Start.Year(), b.Start.Month(), b.Start.Day(), 0, 0, 0, 0, b.Bot.timezone)

	if start.Equal(today) {
		dateString = "Today from "
	} else if start.Equal(today.AddDate(0, 0, 1)) {
		dateString = "Tomorrow from "
	} else {
		dateString = b.Start.Format("Sat Mar 7 from ")
	}

	return dateString + b.Start.Format("3:04") + " to " + b.End.Format("3:04pm")
}

// Creates a slack attachment from this booking
func (b *Booking) AsAttachment(title string, color string) (attachment slack.Attachment) {
	return slack.Attachment{
		Color: color,
		Fields: []slack.AttachmentField{
			slack.AttachmentField{Title: title, Value: b.TimeString(), Short: true},
			slack.AttachmentField{Title: "Who/What", Value: b.What, Short: true},
		},
	}
}
