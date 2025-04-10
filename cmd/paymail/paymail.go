package main

import (
	"os"
	"time"

	"github.com/bitcoin-sv/go-paymail/logging"
	"github.com/bitcoin-sv/go-paymail/server"
)

func main() {
	logger := logging.GetDefaultLogger()

	sl := server.PaymailServiceLocator{}
	sl.RegisterPaymailService(new(OpnsServiceProvider))
	sl.RegisterPikeContactService(new(OpnsServiceProvider))
	sl.RegisterPikePaymentService(new(OpnsServiceProvider))

	var err error
	port := 3001
	// portEnv := os.Getenv("PORT")
	// if portEnv != "" {
	// 	if port, err = strconv.Atoi(portEnv); err != nil {
	// 		logger.Fatal().Msg(err.Error())
	// 	}
	// }
	// Custom server with lots of customizable goodies
	config, err := server.NewConfig(
		&sl,
		server.WithBasicRoutes(),
		server.WithP2PCapabilities(),
		server.WithBeefCapabilities(),
		// server.WithDomain("1sat.app"),
		server.WithDomain(os.Getenv("PAYMAIL_DOMAIN")),
		// server.WithDomain("localhost:3000"),
		// server.WithGenericCapabilities(),
		server.WithPort(port),
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
