package main

import (
	"sync"
)

var wg sync.WaitGroup

func main() {
	wg.Add(1)

	// start smpp server
	smsc := NewSmsc()
	go smsc.Start(wg)

	wg.Wait()
}
