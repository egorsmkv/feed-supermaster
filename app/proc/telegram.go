package proc

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	log "github.com/go-pkgz/lgr"
	"github.com/pkg/errors"
	tb "gopkg.in/telebot.v3"

	"github.com/umputun/feed-master/app/feed"
)

// TelegramClient client
type TelegramClient struct {
	TelegramSender TelegramSender
	Bot            *tb.Bot
	Timeout        time.Duration
}

// TelegramSender is the interface for sending messages to telegram
type TelegramSender interface {
	Send(tb.Audio, *tb.Bot, tb.Recipient, *tb.SendOptions) (*tb.Message, error)
}

// NewTelegramClient init telegram client
func NewTelegramClient(token, apiURL string, timeout time.Duration, tgs TelegramSender) (*TelegramClient, error) {
	log.Printf("[INFO] create telegram client for %s, timeout: %s", apiURL, timeout)
	if timeout == 0 {
		timeout = time.Second * 60
	}

	if token == "" {
		return &TelegramClient{
			Bot:     nil,
			Timeout: timeout,
		}, nil
	}

	bot, err := tb.NewBot(tb.Settings{
		URL:    apiURL,
		Token:  token,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
		Client: &http.Client{Timeout: timeout},
	})
	if err != nil {
		return nil, err
	}

	bot.Handle("/chat_id", func(c tb.Context) error {
		chatID := fmt.Sprintf("%d", c.Chat().ID)

		return c.Send(chatID)
	})

	go bot.Start()

	result := TelegramClient{
		Bot:            bot,
		Timeout:        timeout,
		TelegramSender: tgs,
	}
	return &result, err
}

// Send message, skip if telegram token empty
func (client TelegramClient) Send(channelID string, feed feed.Rss2, item feed.Item) (err error) {
	if client.Bot == nil || channelID == "" {
		return nil
	}

	message, err := client.sendText(channelID, feed, item)
	if err != nil {
		return errors.Wrapf(err, "can't send to telegram for %+v", item.Enclosure)
	}

	log.Printf("[DEBUG] telegram message sent: \n%s", message.Text)
	return nil
}

func (client TelegramClient) sendText(channelID string, feed feed.Rss2, item feed.Item) (*tb.Message, error) {
	message, err := client.Bot.Send(
		recipient{chatID: channelID},
		client.getMessageHTML(feed, item),
		tb.ModeHTML,
		tb.NoPreview,
	)

	return message, err
}

// getMessageHTML generates HTML message from provided feed.Item
func (client TelegramClient) getMessageHTML(feed feed.Rss2, item feed.Item) string {
	var header, footer string
	title := strings.TrimSpace(item.Title)
	if title != "" && item.Link == "" {
		header = fmt.Sprintf("%s\n\n", title)
	} else if title != "" && item.Link != "" {
		header = fmt.Sprintf("<a href=%q>%s</a>\n\n", item.Link, title)
	}

	feed.Title = strings.TrimSpace(feed.Title)
	feedTitle := fmt.Sprintf("<b>%s</b>\n\n", feed.Title)

	return feedTitle + header + footer
}

type recipient struct {
	chatID string
}

func (r recipient) Recipient() string {
	if _, err := strconv.ParseInt(r.chatID, 10, 64); err != nil && !strings.HasPrefix(r.chatID, "@") {
		return "@" + r.chatID
	}

	return r.chatID
}

// TelegramSenderImpl is a TelegramSender implementation that sends messages to Telegram for real
type TelegramSenderImpl struct{}

// Send sends a message to Telegram
func (tg *TelegramSenderImpl) Send(audio tb.Audio, bot *tb.Bot, rcp tb.Recipient, opts *tb.SendOptions) (*tb.Message, error) {
	return audio.Send(bot, rcp, opts)
}
