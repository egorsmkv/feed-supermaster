// Package config provides the configuration support for the application.
package config

import (
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/umputun/feed-master/app/feed"
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
	TelegramGroupID string   `yaml:"telegram_group_id"`
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

// SetDefaults sets default values for config
func (c *Conf) setDefaults() {}
