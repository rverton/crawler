// crawl: simple web-crawler
// crawls specified url up to specified depth and outputs formatted tree-like json representation of result to stdout
// any errors are reported to stderr
// doesn't scan same urls twice, doesn't scan urls from different domains (unless told to)
package main

import (
	"code.google.com/p/go.net/html"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
)

var (
	rootUrl     = flag.String("u", "http://golang.org/", "url to crawl")
	maxDepth    = flag.Int("d", 1, "depth of crawling")
	nworkers    = flag.Int("w", 2, "number of concurrent workers")
	scanForeign = flag.Bool("f", false, "scan urls with different hostname")
)

// helper type for json pretty-printing
type urlp struct {
	*url.URL
}

func (u urlp) MarshalJSON() ([]byte, error) {
	return []byte("\"" + u.String() + "\""), nil
}

type page struct {
	URL        urlp    `json:"url"`
	Depth      int     `json:"depth"`
	Links      []*page `json:"links,omitempty"` // all links on this page
	scanned    bool    // flag for filtering duplicates and not-scannable urls (different domain, invalid urls)
	sync.Mutex `json:"-"`
}

// get next not scanned page within maxDepth, nil if not found
func (p *page) nextPage() *page {
	if !p.scanned && p.Depth < *maxDepth {
		return p
	}
	var next *page
	for _, v := range p.Links {
		if next = v.nextPage(); next != nil {
			return next
		}
	}
	return nil
}

// find first occurance of page with given url, nil if not found
func (p *page) lookup(URL *url.URL) *page {
	if p.URL.String() == URL.String() {
		return p
	}
	var next *page
	for _, v := range p.Links {
		if next = v.lookup(URL); next != nil {
			return next
		}
	}
	return nil
}

// grab scannable pages from root, scan them and update the tree, exit if nextPage returns nil
func (root *page) crawl(wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		// get next page to crawl and set scanned flag
		root.Lock()
		// if no more pages to crawl, return
		p := root.nextPage()
		if p == nil {
			root.Unlock()
			return
		}
		p.scanned = true
		root.Unlock()

		urls := p.scan()

		// update p with retreived urls
		root.Lock()
		var newp *page
		for _, urlStr := range urls {
			// parse the url, try to make it absolute if possible
			urlObj, err := url.Parse(urlStr)
			if err == nil && !urlObj.IsAbs() {
				urlObj, err = p.URL.Parse(urlStr)
			}
			newp = &page{
				URL:   urlp{urlObj},
				Depth: p.Depth + 1,
				Links: make([]*page, 0),
			}
			// if already in the tree or not a valid url, don't mark for scanning
			if root.lookup(urlObj) != nil || err != nil {
				newp.scanned = true
			}
			if !*scanForeign && (urlObj.Host != p.URL.Host) {
				newp.scanned = true
			}
			p.Links = append(p.Links, newp)
		}
		root.Unlock()
	}
}

// fetch the page, parse it and retreive all urls from <a> tags (href attribute). always returns non-nil slice
func (p *page) scan() []string {
	urls := make([]string, 0)
	if p.URL.Scheme == "" {
		p.URL.Scheme = "http"
	}
	resp, err := http.Get(p.URL.String())
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

func main() {
	wg := &sync.WaitGroup{}

	root := &page{
		Links: make([]*page, 0),
	}
	rooturl, err := url.Parse(*rootUrl)
	if err != nil {
		log.Println(err)
		return
	}
	root.URL = urlp{rooturl}

	for i := 0; i < *nworkers; i++ {
		wg.Add(1)
		go root.crawl(wg)
	}
	wg.Wait()

	tree, err := json.MarshalIndent(*root, "", "\t")
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Println(string(tree))
}
