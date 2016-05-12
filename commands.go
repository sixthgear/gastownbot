package main

import (
	"fmt"
	"time"

	"github.com/nlopes/slack"
)

// Command to book a meeting room
func (bot *GastownBot) Book(channel string, username string, text string) (msg slack.Msg) {

	text = username + ": " + text
	call := bot.gcal.Events.QuickAdd(bot.config.CalendarId, text)
	event, err := call.Do()
	if err != nil {
		msg.Text = fmt.Sprintf("Could not book that!")
		return msg
	}

	b := BookingFromEvent(bot, bot.config.CalendarId, event)
	defer bot.SyncBookings()

	msg.Text = fmt.Sprintf("Booked for %s.", b.TimeString())
	return msg
}

// Command to show upcoming bookings
func (bot *GastownBot) Show(channel string, username string) (msg slack.Msg) {

	today := time.Now()
	tomorrow := today.AddDate(0, 0, 1)

	if len(bot.bookingsList) > 0 {
		msg.Text = "Upcoming Bookings:"
		msg.Attachments = append(msg.Attachments, bot.AttachmentsForDay("#3BCBFF", today)...)
		msg.Attachments = append(msg.Attachments, bot.AttachmentsForDay("#33FF3D", tomorrow)...)
		return msg
	} else {
		msg.Text = "No bookings to show."
		return msg
	}
}

// Grab the bookings for the day and send them to channels
func (bot *GastownBot) AttachmentsForDay(color string, t time.Time) (attachments []slack.Attachment) {

	y, m, d := t.Date()
	startTime := time.Date(y, m, d, 0, 0, 0, 0, bot.timezone)
	endTime := startTime.AddDate(0, 0, 1)

	for id, b := range bot.bookingsList {
		if b.IsWithin(startTime, endTime) {
			title := fmt.Sprintf("Booking #%d", id+1)
			attachments = append(attachments, b.AsAttachment(title, color))
		}
	}

	return attachments

}

// Print help text to channel
func (bot *GastownBot) Help(channel string, username string) (msg slack.Msg) {
	// TODO: print help text
	msg.Text = "Booking command syntax:"
	msg.Attachments = []slack.Attachment{
		slack.Attachment{
			// Title: "Command Syntax:",
			Fields: []slack.AttachmentField{
				slack.AttachmentField{
					Value: "/book show",
					Short: true,
				},
				slack.AttachmentField{
					Value: "Show upcoming bookings",
					Short: true,
				},
				slack.AttachmentField{
					Value: "/book 1pm meeting",
					Short: true,
				},
				slack.AttachmentField{
					Value: "Book a meeting for 1pm",
					Short: true,
				},
			},
		},
	}

	return msg
}

func (bot *GastownBot) NotUnderstood(channel string, username string) (msg slack.Msg) {
	return msg
}
