package main

import (
	"fmt"
	"time"

	"github.com/nlopes/slack"
)

type SlashMsgResponse struct {
	*slack.Msg
	ResponseType string `json:"response_type,omitempty"`
}

// Command to book a meeting room
func (bot *GastownBot) Book(channel string, username string, text string) (msg slack.Msg, public bool) {

	if text == "" {
		text = username
	} else {
		text = username + ": " + text
	}

	call := bot.gcal.Events.QuickAdd(bot.config.CalendarId, text)
	event, err := call.Do()
	if err != nil {
		msg.Text = fmt.Sprintf("Couldn't book that!")
		return msg, false
	}

	b := BookingFromEvent(bot, bot.config.CalendarId, event)
	defer bot.SyncBookings()

	msg.Text = " "
	msg.Attachments = append(msg.Attachments, b.AsAttachment("Meeting Room Booked", "#3BCBFF"))
	return msg, true
}

// Command to show upcoming bookings
func (bot *GastownBot) List(channel string, username string) (msg slack.Msg) {

	today := time.Now()
	tomorrow := today.AddDate(0, 0, 1)

	if len(bot.bookingsList) > 0 {
		msg.Text = "Upcoming Meeting Room Bookings:"
		msg.Attachments = append(msg.Attachments, bot.AttachmentsForDay("#3BCBFF", today)...)
		msg.Attachments = append(msg.Attachments, bot.AttachmentsForDay("#33FF3D", tomorrow)...)
		msg.Attachments = append(msg.Attachments, slack.Attachment{
			Text: fmt.Sprintf("<%s|Full Calendar...>", GetCalendarAddress(bot.config.CalendarId)),
		})
		msg.Attachments = append(msg.Attachments, bot.HelpAttachment())
		return msg
	} else {
		msg.Text = "No bookings today."
		msg.Attachments = append(msg.Attachments, bot.HelpAttachment())
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
	msg.Text = " "
	msg.Attachments = append(msg.Attachments, bot.HelpAttachment())
	return msg
}

func (bot *GastownBot) HelpAttachment() slack.Attachment {
	return slack.Attachment{
		Title: "Usage Examples:",
		Fields: []slack.AttachmentField{
			slack.AttachmentField{Value: "/book", Short: true},
			slack.AttachmentField{Value: "_Show me a list of upcoming bookings._", Short: true},
			slack.AttachmentField{Value: "/book 1pm farts", Short: true},
			slack.AttachmentField{Value: "_Book the meeting room for an hour at 1pm._", Short: true},
			slack.AttachmentField{Value: "/book 2-2:30 skype call", Short: true},
			slack.AttachmentField{Value: "_Book the meeting room for 30 minutes._", Short: true},
		},
		MarkdownIn: []string{"text", "fields"},
	}
}

func (bot *GastownBot) NotUnderstood(channel string, username string) (msg slack.Msg) {
	return msg
}
