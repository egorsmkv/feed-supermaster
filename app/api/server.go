// Package api provides rest-like server
package api

import (
	"context"
	"embed"
	"encoding/xml"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/didip/tollbooth/v7"
	"github.com/didip/tollbooth_chi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/go-pkgz/lcw/v2"
	log "github.com/go-pkgz/lgr"
	"github.com/go-pkgz/rest"
	"github.com/go-pkgz/rest/logger"
	"github.com/pkg/errors"

	"github.com/umputun/feed-master/app/config"
	"github.com/umputun/feed-master/app/feed"
)

// Server provides HTTP API
type Server struct {
	Version       string
	Conf          config.Conf
	Store         Store
	TemplLocation string
	AdminPasswd   string

	httpServer *http.Server
	cache      lcw.LoadingCache[[]byte]
	templates  *template.Template
}

// Store provides access to feed data
type Store interface {
	Load(fmFeed string, max int, skipJunk bool) ([]feed.Item, error)
}

// Run starts http server for API with all routes
func (s *Server) Run(ctx context.Context, port int) {
	log.Printf("[INFO] starting server on port %d", port)
	var err error
	o := lcw.NewOpts[[]byte]()
	if s.cache, err = lcw.NewExpirableCache(o.TTL(time.Minute*3), o.MaxCacheSize(10*1024*1024)); err != nil {
		log.Printf("[PANIC] failed to make loading cache, %v", err)
		return
	}

	serverLock := sync.Mutex{}
	go func() {
		<-ctx.Done()
		serverLock.Lock()
		defer serverLock.Unlock()
		if s.httpServer != nil {
			if clsErr := s.httpServer.Close(); clsErr != nil {
				log.Printf("[ERROR] failed to close proxy http server, %v", clsErr)
			}
		}
	}()

	// Parse the templates from the embedded file system
	tmpl, err := template.ParseFS(templatesFS, "templates/*")
	if err != nil {
		log.Printf("[ERROR] failed to parse templates, %v", err)
		return
	}

	s.templates = tmpl

	serverLock.Lock()
	s.httpServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           s.router(),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      s.Conf.System.HTTPResponseTimeout,
		IdleTimeout:       30 * time.Second,
	}
	serverLock.Unlock()
	err = s.httpServer.ListenAndServe()
	log.Printf("[WARN] http server terminated, %s", err)
}

func (s *Server) router() *chi.Mux {
	router := chi.NewRouter()
	router.Use(middleware.RealIP, rest.Recoverer(log.Default()), middleware.GetHead)
	router.Use(middleware.Throttle(1000), middleware.Timeout(60*time.Second))
	router.Use(rest.AppInfo("feed-master", "umputun", s.Version), rest.Ping)
	router.Use(tollbooth_chi.LimitHandler(tollbooth.NewLimiter(5, nil)))

	router.Group(func(rrss chi.Router) {
		l := logger.New(logger.Log(log.Default()), logger.Prefix("[INFO]"), logger.IPfn(logger.AnonymizeIP))
		rrss.Use(l.Handler)
		rrss.Get("/rss/{name}", s.getFeedCtrl)
		rrss.Head("/rss/{name}", s.getFeedCtrl)
		rrss.Get("/list", s.getListCtrl)
		rrss.Get("/feed/{name}", s.getFeedPageCtrl)
		rrss.Get("/feeds", s.getFeedsPageCtrl)
	})

	router.Get("/config", func(w http.ResponseWriter, _ *http.Request) { rest.RenderJSON(w, s.Conf) })

	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		err := tryRead(assetsFS, "static", r.URL.Path, w)
		if err == nil {
			return
		}
	})

	return router
}

// GET /rss/{name} - returns rss for given feeds set
func (s *Server) getFeedCtrl(w http.ResponseWriter, r *http.Request) {
	feedName := chi.URLParam(r, "name")

	data, err := s.cache.Get("feed::"+feedName, func() ([]byte, error) {
		items, err := s.Store.Load(feedName, s.Conf.System.MaxTotal, true)
		if err != nil {
			return nil, err
		}

		for i, itm := range items {
			// add ts suffix to titles
			switch s.Conf.Feeds[feedName].ExtendDateTitle {
			case "yyyyddmm":
				items[i].Title = fmt.Sprintf("%s (%s)", itm.Title, itm.DT.Format("2006-02-01")) // nolint
			case "yyyymmdd":
				items[i].Title = fmt.Sprintf("%s (%s)", itm.Title, itm.DT.Format("2006-01-02"))
			}
		}

		rss := feed.Rss2{
			Version:        "2.0",
			ItemList:       items,
			Title:          s.Conf.Feeds[feedName].Title,
			Description:    s.Conf.Feeds[feedName].Description,
			Language:       s.Conf.Feeds[feedName].Language,
			Link:           s.Conf.Feeds[feedName].Link,
			PubDate:        items[0].PubDate,
			LastBuildDate:  time.Now().Format(time.RFC822Z),
			ItunesAuthor:   s.Conf.Feeds[feedName].Author,
			ItunesExplicit: "no",
			ItunesOwner: &feed.ItunesOwner{
				Name:  "Feed Master",
				Email: s.Conf.Feeds[feedName].OwnerEmail,
			},
			NsItunes: "http://www.itunes.com/dtds/podcast-1.0.dtd",
			NsMedia:  "http://search.yahoo.com/mrss/",
		}

		// replace link to UI page
		if s.Conf.System.BaseURL != "" {
			baseURL := strings.TrimSuffix(s.Conf.System.BaseURL, "/")
			rss.Link = baseURL + "/feed/" + feedName
			imagesURL := baseURL + "/images/" + feedName
			rss.ItunesImage = &feed.ItunesImg{URL: imagesURL}
			rss.MediaThumbnail = &feed.MediaThumbnail{URL: imagesURL}
		}

		b, err := xml.MarshalIndent(&rss, "", "  ")
		if err != nil {
			rest.SendErrorJSON(w, r, log.Default(), http.StatusInternalServerError, err, "failed to marshal rss")
			return nil, errors.Wrapf(err, "failed to marshal rss for %s", feedName)
		}

		res := `<?xml version="1.0" encoding="UTF-8"?>` + "\n" + string(b)

		// this hack to avoid having different items for marshal and unmarshal due to "itunes" namespace
		res = strings.Replace(res, "<duration>", "<itunes:duration>", -1)
		res = strings.Replace(res, "</duration>", "</itunes:duration>", -1)

		return []byte(res), nil
	})

	if err != nil {
		rest.SendErrorJSON(w, r, log.Default(), http.StatusBadRequest, err, "failed to get feed")
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=UTF-8")
	_, _ = fmt.Fprintf(w, "%s", data)
}

// GET /list - returns feed's image
func (s *Server) getListCtrl(w http.ResponseWriter, r *http.Request) {
	feeds := s.feeds()
	render.JSON(w, r, feeds)
}

func (s *Server) feeds() []string {
	feeds := make([]string, 0, len(s.Conf.Feeds))
	for k := range s.Conf.Feeds {
		feeds = append(feeds, k)
	}
	return feeds
}

func tryRead(fs embed.FS, prefix, requestedPath string, w http.ResponseWriter) error {
	f, err := fs.Open(path.Join(prefix, requestedPath))
	if err != nil {
		return err
	}
	defer f.Close()

	stat, _ := f.Stat()
	if stat.IsDir() {
		return errors.New("path is dir")
	}

	contentType := mime.TypeByExtension(filepath.Ext(requestedPath))
	w.Header().Set("Content-Type", contentType)
	_, err = io.Copy(w, f)

	return err
}
