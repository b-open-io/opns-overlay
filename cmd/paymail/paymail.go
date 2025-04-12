package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/bitcoin-sv/go-paymail/logging"
	"github.com/bitcoin-sv/go-paymail/server"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load("../../.env")
	logger := logging.GetDefaultLogger()

	sl := server.PaymailServiceLocator{}
	sl.RegisterPaymailService(new(OpnsServiceProvider))
	sl.RegisterPikeContactService(new(OpnsServiceProvider))
	sl.RegisterPikePaymentService(new(OpnsServiceProvider))

	var err error
	PORT, _ := strconv.Atoi(os.Getenv("PAYMAIL_PORT"))
	flag.IntVar(&PORT, "p", PORT, "Port to listen on")
	flag.Parse()
	if PORT == 0 {
		PORT = 3001
	}

	paymailDomain := os.Getenv("PAYMAIL_DOMAIN")
	if paymailDomain == "" {
		paymailDomain = fmt.Sprintf("localhost:%d", PORT)
	}
	// Custom server with lots of customizable goodies
	config, err := server.NewConfig(
		&sl,
		server.WithBasicRoutes(),
		server.WithP2PCapabilities(),
		server.WithBeefCapabilities(),
		// server.WithDomain("1sat.app"),
		server.WithDomain(paymailDomain),
		// server.WithDomain("localhost:3000"),
		// server.WithGenericCapabilities(),
		server.WithPort(PORT),
		// server.WithServiceName("BsvAliasCustom"),
		server.WithTimeout(15*time.Second),
		// server.WithCapabilities(customCapabilities()),
	)
	if err != nil {
		logger.Fatal().Msg(err.Error())
	}
	config.Prefix = "https://" //normally paymail requires https, but for demo purposes we'll use http

	// Create & start the server
	server.StartServer(server.CreateServer(config), config.Logger)

}
