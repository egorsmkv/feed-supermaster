package feed

import (
	"html/template"
	"time"
)

// Item for rss
type Item struct {
	// Internal
	DT time.Time `xml:"-"`
	// Required
	Title       string        `xml:"title"`
	Link        string        `xml:"link"`
	Description template.HTML `xml:"description"`
	GUID        string        `xml:"guid"`
	// Optional
	Content     template.HTML `xml:"encoded,omitempty"`
	PubDate     string        `xml:"pubDate,omitempty"`
	Comments    string        `xml:"comments,omitempty"`
	Author      string        `xml:"author,omitempty"`
	Duration    string        `xml:"duration,omitempty"`
	DurationFmt string        `xml:"-"` // used for ui only in
	Enclosure   Enclosure     `xml:"enclosure"`
	Junk        bool          `xml:"-"`
}
