package main

import (
	"log"
	"os"
	"strconv"
	"sync"
)

var wg sync.WaitGroup

func main() {
	smscPort := 2775
	smscPortStr := os.Getenv("SMSC_PORT")
	if smscPortStr != "" {
		p, err := strconv.Atoi(smscPortStr)
		if err != nil || p < 1 {
			log.Fatalf("invalid SMSC_PORT [%s]", smscPortStr)
		} else {
			smscPort = p
		}
	}

	wg.Add(1)

	// start smpp server
	smsc := NewSmsc()
	go smsc.Start(smscPort, wg)

	wg.Wait()
}
