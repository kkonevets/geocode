package main

import (
	"github.com/X-Company/geocoding/cache/server"
	"github.com/X-Company/geocoding/config"
	_ "github.com/lib/pq"
)

func main() {
	config.InitLogging("cache.log")
	server.StartServer()
}
