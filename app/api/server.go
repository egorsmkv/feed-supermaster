// Package api provides rest-like server
package api

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-pkgz/lcw/v2"
	log "github.com/go-pkgz/lgr"
	"github.com/go-pkgz/rest/logger"
	"github.com/pkg/errors"

	"github.com/umputun/feed-master/app/config"
	"github.com/umputun/feed-master/app/feed"
)

// Server provides HTTP API
type Server struct {
	Store Store
	cache lcw.LoadingCache[[]byte]

	httpServer    *http.Server
	templates     *template.Template
	Version       string
	TemplLocation string
	Conf          config.Conf
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

	router.Group(func(r chi.Router) {
		l := logger.New(
			logger.Log(log.Default()),
			logger.Prefix("[INFO]"),
		)

		r.Use(l.Handler)

		r.Get("/feed/{name}", s.getFeedPageCtrl)
		r.Get("/feeds", s.getFeedsPageCtrl)
	})

	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		err := tryRead(assetsFS, "static", r.URL.Path, w)
		if err == nil {
			return
		}
	})

	return router
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
	if stat == nil {
		return errors.New("can't stat file")
	}
	if stat.IsDir() {
		return errors.New("path is dir")
	}

	contentType := mime.TypeByExtension(filepath.Ext(requestedPath))
	w.Header().Set("Content-Type", contentType)
	_, err = io.Copy(w, f)

	return err
}
