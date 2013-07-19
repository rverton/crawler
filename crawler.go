// Crawling package
package crawler

import (
	"code.google.com/p/go.net/html"
	"log"
	"net/http"
	"net/url"
	"sync"
)

const (
	WORKERS = 4 // Concurrent workers
)

var (
	in  chan string
	out chan []string
)

type site struct {
	url   *url.URL
	Depth int
	Links map[string]link
	sync.Mutex
}

type link struct {
	scanned bool
	depth   int
}

// Get next link from list
func (s *site) next() (*string, link) {
	var url string
	for k, v := range s.Links {
		if !v.scanned && v.depth <= s.Depth {
			url = k
			return &url, v
		}
	}
	return nil, link{}
}

// Extract all links from a page
func scan(urlString string) []string {
	u, err := url.Parse(urlString)

	if err != nil {
		return []string{}
	}

	urls := make([]string, 0)
	if u.Scheme == "" {
		u.Scheme = "http"
	}
	resp, err := http.Get(u.String())
	if err != nil {
		log.Println(err)
		return urls
	}
	defer resp.Body.Close()

	t := html.NewTokenizer(resp.Body)

	for tok := t.Next(); tok != html.ErrorToken; tok = t.Next() {
		if tok != html.StartTagToken {
			continue
		}
		if tname, hasAttr := t.TagName(); len(tname) > 1 || tname[0] != 'a' || !hasAttr {
			continue
		}
		// at this point we definately have "a" tag with _some_ attributes
		for attr, val, more := t.TagAttr(); ; attr, val, more = t.TagAttr() {
			if string(attr) == "href" {
				urls = append(urls, string(val))
				break
			}
			if !more {
				break
			}
		}
	}
	return urls
}

// Crawl worker, gets next link from pool and extracs urls
func (root *site) crawl(wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		root.Lock()

		// No more pages in pool?
		scanUrl, scanLink := root.next()
		if scanUrl == nil {
			root.Unlock()
			return
		}

		scanLink.scanned = true

		root.Links[*scanUrl] = scanLink
		root.Unlock()

		urls := scan(*scanUrl)
		root.Lock()
		for _, v := range urls {

			urlObj, err := url.Parse(v)
			if err != nil {
				continue
			}

			// Strip anchor
			urlObj.Fragment = ""

			if !urlObj.IsAbs() {
				urlObj, err = root.url.Parse(urlObj.String())

				if err != nil {
					continue
				}
			}

			// Link already scanned?
			if _, ok := root.Links[urlObj.String()]; ok {
				continue
			}

			if root.url.Host == urlObj.Host {
				l := link{scanned: false, depth: scanLink.depth + 1}
				root.Links[urlObj.String()] = l
			}

		}
		root.Unlock()

	}
}

func newRootSite(urlString string, depth int) *site {
	root := &site{}

	urlObj, err := url.Parse(urlString)
	if err != nil {
		log.Printf("Error parsing: %v", urlString)
		return nil
	}

	root.url = urlObj
	root.Depth = depth

	links := make(map[string]link, 0)
	links[urlString] = link{scanned: false, depth: 0}
	root.Links = links

	return root
}

// Returns in and out channel to schedule a crawling process.
func Start() (chan string, chan []string) {

	buffer := 5

	in = make(chan string, buffer)
	out = make(chan []string, buffer)

	// Wait for incoming urls and push result to out
	scheduler := func() {

		for v := range in {
			wg := &sync.WaitGroup{}

			root := newRootSite(v, 1)

			for i := 0; i < WORKERS; i++ {
				wg.Add(1)
				go root.crawl(wg)
			}

			wg.Wait()

			urls := make([]string, 0)

			for k, _ := range root.Links {
				urls = append(urls, k)
			}

			out <- urls
		}
	}

	go scheduler()

	return in, out

}
