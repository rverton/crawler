package main

import (
	"code.google.com/p/go.net/html"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

const (
	NWORKERS = 2
	MAXDEPTH = 2
)

type page struct {
	URL      string
	Depth    int
	canCrawl bool
	Links    []*page
	sync.Mutex
}

func (p *page) nextPage() *page {
	if p.canCrawl && p.Depth < MAXDEPTH {
		return p
	}
	for _, v := range p.Links {
		if next := v.nextPage(); next != nil {
			return next
		}
	}
	return nil
}

func (p *page) lookup(URL string) *page {
	if p.URL == URL {
		return p
	}
	for _, v := range p.Links {
		if next := v.lookup(URL); next != nil {
			return next
		}
	}
	return nil
}

func (root *page) crawl(wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		root.Lock()
		// if no more pages to crawl, return
		p := root.nextPage()
		if p == nil {
			root.Unlock()
			return
		}
		p.canCrawl = false
		root.Unlock()

		fmt.Println("scanning", p.URL)

		urls := p.scan()

		root.Lock()
		for _, URL := range urls {
			newp := &page{
				URL:      URL,
				Depth:    p.Depth + 1,
				canCrawl: true,
				Links:    make([]*page, 0),
			}
			if root.lookup(URL) != nil {
				newp.canCrawl = false
			}
			p.Links = append(p.Links, newp)
		}
		root.Unlock()
	}
}

func (p *page) scan() []string {
	urls := make([]string, 0)
	resp, err := http.Get(p.URL)
	if err != nil {
		fmt.Println(err)
		return urls
	}
	defer resp.Body.Close()

	t := html.NewTokenizer(resp.Body)

	for tok := t.Next(); tok != html.ErrorToken; tok = t.Next() {
		if tok != html.StartTagToken {
			continue
		}
		if tname, hasAttr := t.TagName(); !hasAttr || len(tname) > 1 || tname[0] != 'a' {
			continue
		}
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
		URL:      "http://golang.org",
		canCrawl: true,
		Links:    make([]*page, 0),
	}

	for i := 0; i < NWORKERS; i++ {
		wg.Add(1)
		go root.crawl(wg)
	}
	wg.Wait()
	tree, err := json.MarshalIndent(*root, "", "\t")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(tree))
}
