package api

import (
	"bytes"
	"net/http"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/go-pkgz/rest"
	"github.com/pkg/errors"

	"github.com/umputun/feed-master/app/config"
	"github.com/umputun/feed-master/app/feed"
)

// GET /feed/{name} - renders page with list of items
func (s *Server) getFeedPageCtrl(w http.ResponseWriter, r *http.Request) {
	if s.cache == nil {
		s.renderErrorPage(w, r, errors.New("cache not initialized"), 500)
		return
	}

	feedName := chi.URLParam(r, "name")

	data, err := s.cache.Get(feedName, func() ([]byte, error) {
		items, err := s.Store.Load(feedName, s.Conf.System.MaxTotal, false)
		if err != nil {
			return nil, err
		}

		// fill formatted duration
		for i, item := range items { //nolint
			if item.Duration == "" {
				continue
			}
			d, e := time.ParseDuration(item.Duration + "s")
			if e != nil {
				continue
			}
			items[i].DurationFmt = d.String()
		}

		tmplData := struct {
			LastUpdate      time.Time
			Name            string
			Description     string
			Link            string
			SinceLastUpdate string
			Version         string
			RSSLink         string
			SourcesLink     string
			TelegramGroupID string
			Items           []feed.Item
			Feeds           int
		}{
			Items:           items,
			Name:            s.Conf.Feeds[feedName].Title,
			Description:     s.Conf.Feeds[feedName].Description,
			Link:            s.Conf.Feeds[feedName].Link,
			LastUpdate:      items[0].DT.In(time.UTC),
			SinceLastUpdate: humanize.Time(items[0].DT),
			Feeds:           len(s.Conf.Feeds[feedName].Sources),
			Version:         s.Version,
			RSSLink:         s.Conf.System.BaseURL + "/rss/" + feedName,
			SourcesLink:     s.Conf.System.BaseURL + "/feed/" + feedName + "/sources",
			TelegramGroupID: s.Conf.Feeds[feedName].TelegramGroupID,
		}

		res := bytes.NewBuffer(nil)
		err = s.templates.ExecuteTemplate(res, "feed.tmpl", &tmplData)
		return res.Bytes(), err
	})
	if err != nil {
		s.renderErrorPage(w, r, err, 400)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// GET /feeds - renders page with list of feeds
func (s *Server) getFeedsPageCtrl(w http.ResponseWriter, r *http.Request) {
	if s.cache == nil {
		s.renderErrorPage(w, r, errors.New("cache not initialized"), 500)
		return
	}

	data, err := s.cache.Get("feeds", func() ([]byte, error) {
		feeds := s.feeds()

		type feedItem struct {
			LastUpdated time.Time
			FeedURL     string
			SourcesLink string
			config.Feed
			Sources int
		}
		var feedItems []feedItem
		for _, f := range feeds {
			items, loadErr := s.Store.Load(f, s.Conf.System.MaxTotal, true)
			if loadErr != nil {
				continue
			}
			feedConf := s.Conf.Feeds[f]
			item := feedItem{
				Feed:        feedConf,
				FeedURL:     s.Conf.System.BaseURL + "/feed/" + f,
				Sources:     len(feedConf.Sources),
				SourcesLink: s.Conf.System.BaseURL + "/feed/" + f + "/sources",
				LastUpdated: items[0].DT.In(time.UTC),
			}
			feedItems = append(feedItems, item)
		}

		tmplData := struct {
			Feeds      []feedItem
			FeedsCount int
		}{
			Feeds:      feedItems,
			FeedsCount: len(feedItems),
		}

		res := bytes.NewBuffer(nil)
		err := s.templates.ExecuteTemplate(res, "feeds.tmpl", &tmplData)
		return res.Bytes(), err
	})
	if err != nil {
		s.renderErrorPage(w, r, err, 400)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) renderErrorPage(w http.ResponseWriter, r *http.Request, err error, errCode int) {
	tmplData := struct {
		Error  string
		Status int
	}{Status: errCode, Error: err.Error()}

	if err := s.templates.ExecuteTemplate(w, "error.tmpl", &tmplData); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, rest.JSON{"error": err.Error()})
		return
	}
}
