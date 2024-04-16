// Package config provides the configuration support for the application.
package config

import (
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/umputun/feed-master/app/feed"
	"github.com/umputun/feed-master/app/youtube"
)

// Conf for feeds config yml
type Conf struct {
	Feeds  map[string]Feed `yaml:"feeds"`
	System struct {
		UpdateInterval      time.Duration `yaml:"update"`
		HTTPResponseTimeout time.Duration `yaml:"http_response_timeout"`
		MaxItems            int           `yaml:"max_per_feed"`
		MaxTotal            int           `yaml:"max_total"`
		MaxKeepInDB         int           `yaml:"max_keep"`
		Concurrent          int           `yaml:"concurrent"`
		BaseURL             string        `yaml:"base_url"`
	} `yaml:"system"`

	YouTube struct {
		DlTemplate      string             `yaml:"dl_template"`
		BaseChanURL     string             `yaml:"base_chan_url"`
		BasePlaylistURL string             `yaml:"base_playlist_url"`
		Channels        []youtube.FeedInfo `yaml:"channels"`
		BaseURL         string             `yaml:"base_url"`
		UpdateInterval  time.Duration      `yaml:"update"`
		MaxItems        int                `yaml:"max_per_channel"`
		FilesLocation   string             `yaml:"files_location"`
		RSSLocation     string             `yaml:"rss_location"`
		SkipShorts      time.Duration      `yaml:"skip_shorts"`
		DisableUpdates  bool               `yaml:"disable_updates"`
	} `yaml:"youtube"`
}

// Source defines config section for source
type Source struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

// Feed defines config section for a feed~
type Feed struct {
	Title           string   `yaml:"title"`
	Description     string   `yaml:"description"`
	Link            string   `yaml:"link"`
	Image           string   `yaml:"image"`
	Language        string   `yaml:"language"`
	TelegramChannel string   `yaml:"telegram_channel"`
	Filter          Filter   `yaml:"filter"`
	Sources         []Source `yaml:"sources"`
	ExtendDateTitle string   `yaml:"ext_date"`
	Author          string   `yaml:"author"`
	OwnerEmail      string   `yaml:"owner_email"`
}

// Filter defines feed section for a feed filter~
type Filter struct {
	Title  string `yaml:"title"`
	Invert bool   `yaml:"invert"`
}

// Skip items with this regexp
func (filter *Filter) Skip(item feed.Item) (bool, error) {
	mayInvert := func(b bool) bool {
		if filter.Invert {
			return !b
		}
		return b
	}

	if filter.Title != "" {
		matched, err := regexp.MatchString(filter.Title, item.Title)
		if err != nil {
			return mayInvert(matched), err
		}
		return mayInvert(matched), nil
	}
	return false, nil
}

// YTChannel defines youtube channel config
type YTChannel struct {
	ID   string
	Name string
}

// Load config from file
func Load(fname string) (res *Conf, err error) {
	res = &Conf{}
	data, err := os.ReadFile(fname) // nolint
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, res); err != nil {
		return nil, err
	}
	res.setDefaults()
	return res, nil
}

// SingleFeed returns single feed "fake" config for no-config mode
func SingleFeed(feedURL, ch string, updateInterval time.Duration) *Conf {
	conf := Conf{}
	f := Feed{
		TelegramChannel: ch,
		Sources: []Source{
			{Name: "auto", URL: feedURL},
		},
	}
	conf.Feeds = map[string]Feed{"auto": f}
	conf.System.UpdateInterval = updateInterval
	conf.setDefaults()
	return &conf
}

// SetDefaults sets default values for config
func (c *Conf) setDefaults() {}
