package main

import (
	"flag"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/b-open-io/opns-overlay/opns"
	"github.com/b-open-io/overlay/storage"
	"github.com/b-open-io/overlay/util"
	"github.com/bitcoin-sv/go-paymail/logging"
	"github.com/bitcoin-sv/go-paymail/server"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load("../../.env")
	logger := logging.GetDefaultLogger()
	txStore, err := util.NewRedisTxStorage(os.Getenv("REDIS_BEEF"))
	if err != nil {
		log.Fatalf("Failed to initialize tx storage: %v", err)
	}
	store, err := storage.NewRedisStorage(os.Getenv("REDIS"), txStore)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	lookupService, err := opns.NewLookupService(
		os.Getenv("REDIS"),
		store,
		"tm_OpNS",
	)
	if err != nil {
		log.Fatalf("Failed to initialize lookup service: %v", err)
	}

	sl := server.PaymailServiceLocator{}
	serviceProvider := &OpnsServiceProvider{
		Lookup: lookupService,
	}
	sl.RegisterPaymailService(serviceProvider)
	sl.RegisterPikeContactService(serviceProvider)
	sl.RegisterPikePaymentService(serviceProvider)

	PORT, _ := strconv.Atoi(os.Getenv("PAYMAIL_PORT"))
	flag.IntVar(&PORT, "p", PORT, "Port to listen on")
	flag.Parse()
	if PORT == 0 {
		PORT = 3001
	}

	paymailDomain := os.Getenv("PAYMAIL_DOMAINS")
	configs := []server.ConfigOps{
		server.WithBasicRoutes(),
		server.WithP2PCapabilities(),
		server.WithBeefCapabilities(),
		server.WithGenericCapabilities(),
		server.WithPort(PORT),
		server.WithTimeout(15 * time.Second),
		// server.WithCapabilities(customCapabilities()),
	}
	for _, domain := range strings.Split(paymailDomain, ",") {
		domain = strings.TrimSpace(domain)
		if len(domain) > 0 {
			configs = append(configs, server.WithDomain(domain))
		}
	}

	// Custom server with lots of customizable goodies
	config, err := server.NewConfig(
		&sl,
		configs...,
	)
	if err != nil {
		logger.Fatal().Msg(err.Error())
	}
	config.Prefix = "https://" //normally paymail requires https, but for demo purposes we'll use http

	// Create & start the server
	server.StartServer(server.CreateServer(config), config.Logger)

}
