package main

import (
	"code.google.com/p/go.net/html"
	//"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
)

const (
	NWORKERS = 2
	MAXDEPTH = 2
)

type page struct {
	URL     *url.URL
	Depth   int
	scanned bool
	Links   []*page
	sync.Mutex
}

func (p *page) nextPage() *page {
	if !p.scanned && p.Depth < MAXDEPTH {
		return p
	}
	for _, v := range p.Links {
		if next := v.nextPage(); next != nil {
			return next
		}
	}
	return nil
}

func (p *page) lookup(URL *url.URL) *page {
	//fmt.Println("\t", p.URL.String(), URL.String(), p.URL.String() == URL.String())
	if p.URL.String() == URL.String() {
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
		p.scanned = true
		root.Unlock()

		fmt.Println("scanning", p.URL)

		urls := p.scan()

		root.Lock()
		for _, urlStr := range urls {
			url, err := url.Parse(urlStr)
			if err == nil && !url.IsAbs() {
				url, err = p.URL.Parse(urlStr)
			}
			newp := &page{
				URL:   url,
				Depth: p.Depth + 1,
				Links: make([]*page, 0),
			}
			// if already in the tree or not a valid url, don't mark for scanning
			if root.lookup(url) != nil || err != nil {
				newp.scanned = true
			}
			p.Links = append(p.Links, newp)
		}
		root.Unlock()
	}
}

func (p *page) scan() []string {
	urls := make([]string, 0)
	resp, err := http.Get(p.URL.String())
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
		Links: make([]*page, 0),
	}
	var err error
	root.URL, err = url.Parse("http://golang.org/")
	if err != nil {
		fmt.Println(err)
		return
	}

	for i := 0; i < NWORKERS; i++ {
		wg.Add(1)
		go root.crawl(wg)
	}
	wg.Wait()
	//tree, err := json.MarshalIndent(*root, "", "\t")
	//if err != nil {
	//fmt.Println(err)
	//return
	//}
	//fmt.Println(string(tree))
}
