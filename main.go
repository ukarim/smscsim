package main

import (
	"log"
	"os"
	"strconv"
	"sync"
)

var wg sync.WaitGroup

func main() {
	smscPort := getPort("SMSC_PORT", 2775)
	webPort := getPort("WEB_PORT", 12775)
	failedDlrs := "true" == os.Getenv("FAILED_DLRS")

	wg.Add(2)

	// start smpp server
	smsc := NewSmsc(failedDlrs)
	go smsc.Start(smscPort, wg)

	// start web server
	webServer := NewWebServer(smsc)
	go webServer.Start(webPort, wg)

	wg.Wait()
}

func getPort(envVar string, defVal int) int {
	port := defVal
	portStr := os.Getenv(envVar)
	if portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil || p < 1 {
			log.Fatalf("invalid port %s [%s]", envVar, portStr)
		} else {
			port = p
		}
	}
	return port
}
