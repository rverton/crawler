package crawler

import (
	"sync"
	"testing"
	"time"
)

const TIMEOUT = 10

func TestScheduler(t *testing.T) {

	wg := sync.WaitGroup{}

	in, out := Start()

	go func() {
		select {
		case res := <-out:
			t.Log(res.Result)
			wg.Done()
		case <-time.After(TIMEOUT * time.Second):
			t.Error("Timeout, no result received")
			wg.Done()
		}
	}()

	wg.Add(1)
	in <- Crawl{URL: "http://golang.org", Depth: 0}

	wg.Wait()
}
