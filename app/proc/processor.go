// Package proc provided the primary blocking loop
// updating from sources and making feeds
package proc

import (
	"context"
	"time"

	log "github.com/go-pkgz/lgr"
	"github.com/go-pkgz/repeater"
	"github.com/go-pkgz/syncs"

	"github.com/umputun/feed-master/app/config"
	"github.com/umputun/feed-master/app/feed"
)

// TelegramNotif is interface to send messages to telegram
type TelegramNotif interface {
	Send(chanID string, feed feed.Rss2, item feed.Item) error
}

// Processor is a feed reader and store writer
type Processor struct {
	Conf          *config.Conf
	Store         *BoltDB
	TelegramNotif TelegramNotif
}

// Do activate loop of goroutine for each feed, concurrency limited by p.Conf.Concurrent
func (p *Processor) Do(ctx context.Context) error {
	log.Printf("[INFO] activate processor, feeds=%d, %+v", len(p.Conf.Feeds), p.Conf)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			p.processFeeds(ctx)
		}
	}
}

func (p *Processor) processFeeds(ctx context.Context) {
	log.Printf("[DEBUG] refresh started")

	swg := syncs.NewSizedGroup(p.Conf.System.Concurrent, syncs.Preemptive, syncs.Context(ctx))
	for name, fm := range p.Conf.Feeds { //nolint
		for _, src := range fm.Sources {
			name, src, fm := name, src, fm
			swg.Go(func(context.Context) {
				p.processFeed(name, src.URL, fm.TelegramGroupID, p.Conf.System.MaxItems, fm.Filter)
			})
		}
	}
	swg.Wait()

	log.Printf("[DEBUG] refresh completed")

	time.Sleep(p.Conf.System.UpdateInterval)
}

func (p *Processor) processFeed(name, url, telegramGroupID string, maxVal int, filter config.Filter) {
	rss, err := feed.Parse(url)
	if err != nil {
		log.Printf("[WARN] failed to parse %s, %v", url, err)
		return
	}

	// up to MaxItems (5) items from each feed
	upto := maxVal
	if len(rss.ItemList) <= maxVal {
		upto = len(rss.ItemList)
	}

	for _, item := range rss.ItemList[:upto] { //nolint
		// skip 1y and older
		if item.DT.Before(time.Now().AddDate(-1, 0, 0)) {
			continue
		}

		skip, err := filter.Skip(item)
		if err != nil {
			log.Printf("[WARN] failed to filter %s (%s) to %s, save as is, %v", item.GUID, item.PubDate, name, err)
		}
		if skip {
			item.Junk = true
			log.Printf("[INFO] filtered %s (%s), %s %s", item.GUID, item.PubDate, name, item.Title)
		}

		created, err := p.Store.Save(name, item)
		if err != nil {
			log.Printf("[WARN] failed to save %s (%s) to %s, %v", item.GUID, item.PubDate, name, err)
		}

		// don't attempt to send anything if the entry was already saved
		// or in case it was filtered out
		if !created || item.Junk {
			continue
		}

		rptr := repeater.NewDefault(3, 5*time.Second)
		err = rptr.Do(context.Background(), func() error {
			if e := p.TelegramNotif.Send(telegramGroupID, rss, item); e != nil {
				log.Printf("[WARN] failed attempt to send telegram message, url=%s to channel=%s, %v",
					item.Enclosure.URL, telegramGroupID, e)
				return err
			}
			return nil
		})
		if err != nil {
			log.Printf("[WARN] failed to send telegram message, to channel=%s, %v", telegramGroupID, err)
		}
	}

	// keep up to MaxKeepInDB items in bucket
	if removed, err := p.Store.removeOld(name, p.Conf.System.MaxKeepInDB); err == nil {
		if removed > 0 {
			log.Printf("[DEBUG] removed %d from %s", removed, name)
		}
	} else {
		log.Printf("[WARN] failed to remove, %v", err)
	}
}
