// Command sonic-all-stars-racing-transformed-3ds runs a community NEX server
// implementation for Sonic & All-Stars Racing Transformed on Nintendo 3DS.
// Its game-specific protocol behavior is experimental; see README.md.
package main

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/Protarium-Network/sonic-all-stars-racing-transformed-3ds/globals"
	"github.com/Protarium-Network/sonic-all-stars-racing-transformed-3ds/nex"
)

func main() {
	secureServerHost := os.Getenv("PN_SONIC3DS_SECURE_SERVER_HOST")
	secureServerPort, _ := strconv.Atoi(os.Getenv("PN_SONIC3DS_SECURE_SERVER_PORT"))

	if secureServerHost == "" || secureServerPort == 0 {
		globals.Logger.Critical("PN_SONIC3DS_SECURE_SERVER_HOST / PN_SONIC3DS_SECURE_SERVER_PORT environment variables not set")
		os.Exit(1)
	}

	nex.SetSecureStationURL(secureServerHost, secureServerPort)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer logPanic("Authentication Server")
		nex.StartAuthenticationServer(&wg)
	}()

	go func() {
		defer logPanic("Secure Server")
		nex.StartSecureServer(&wg)
	}()

	wg.Wait()
}

func logPanic(serverName string) {
	if r := recover(); r != nil {
		globals.Logger.Criticalf("[PANIC] %s crashed: %v", serverName, r)
		fmt.Fprintf(os.Stderr, "[PANIC] %s crashed: %v\n", serverName, r)
		os.Exit(1)
	}
}
