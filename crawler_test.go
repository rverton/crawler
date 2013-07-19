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

	// Timeout for channel
	timeout := make(chan bool, 1)
	go func() {
		time.Sleep(TIMEOUT * time.Second)
		timeout <- true
	}()

	go func() {
		select {
		case <-out:
			t.Log(out)
			wg.Done()
		case <-timeout:
			t.Error("Timeout, no result received")
			wg.Done()
		}
	}()

	wg.Add(1)
	in <- "http://golang.org"

	wg.Wait()
}
