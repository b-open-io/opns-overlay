package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/4chain-ag/go-overlay-services/pkg/appconfig"
	"github.com/4chain-ag/go-overlay-services/pkg/core/engine"
	"github.com/4chain-ag/go-overlay-services/pkg/server"
	"github.com/b-open-io/opns-overlay/opns"
	"github.com/b-open-io/overlay/lookup/events"
	"github.com/b-open-io/overlay/storage"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/broadcaster"
	"github.com/bsv-blockchain/go-sdk/transaction/chaintracker/headers_client"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

var chaintracker headers_client.Client
var PORT int
var SYNC bool
var rdb, sub *redis.Client
var peers = []string{}

type subRequest struct {
	topics []string
	writer *bufio.Writer
}

var subscribe = make(chan *subRequest, 100)   // Buffered channel
var unsubscribe = make(chan *subRequest, 100) // Buffered channel
func init() {
	godotenv.Load("../../.env")
	chaintracker = headers_client.Client{
		Url:    os.Getenv("BLOCK_HEADERS_URL"),
		ApiKey: os.Getenv("BLOCK_HEADERS_API_KEY"),
	}
	PORT, _ = strconv.Atoi(os.Getenv("PORT"))
	flag.IntVar(&PORT, "p", PORT, "Port to listen on")
	flag.BoolVar(&SYNC, "s", false, "Start sync")
	flag.Parse()
	if PORT == 0 {
		PORT = 3000
	}
	if redisOpts, err := redis.ParseURL(os.Getenv("REDIS")); err != nil {
		log.Fatalf("Failed to parse Redis URL: %v", err)
	} else {
		rdb = redis.NewClient(redisOpts)
		sub = redis.NewClient(redisOpts)
	}
	PEERS := os.Getenv("PEERS")
	if PEERS != "" {
		peers = strings.Split(PEERS, ",")
	}
}

func main() {
	// Create a context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Channel to listen for OS signals
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	store, err := storage.NewRedisStorage(os.Getenv("REDIS"))
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	lookupService, err := opns.NewLookupService(
		os.Getenv("REDIS"),
		store,
		"tm_OpNS",
	)
	if err != nil {
		log.Fatalf("Failed to initialize lookup service: %v", err)
	}
	tm := "tm_OpNS"
	e := &engine.Engine{
		Managers: map[string]engine.TopicManager{
			tm: &opns.TopicManager{},
		},
		LookupServices: map[string]engine.LookupService{
			"ls_OpNS": lookupService,
		},
		Storage:           store,
		ChainTracker:      chaintracker,
		SyncConfiguration: map[string]engine.SyncConfiguration{},
		Broadcaster: &broadcaster.Arc{
			ApiUrl:  "https://arc.taal.com/v1",
			WaitFor: broadcaster.ACCEPTED_BY_NETWORK,
		},
		HostingURL:   os.Getenv("HOSTING_URL"),
		PanicOnError: true,
	}

	http, err := server.New(
		server.WithFiberMiddleware(logger.New()),
		server.WithFiberMiddleware(compress.New()),
		server.WithFiberMiddleware(cors.New(cors.Config{AllowOrigins: "*"})),
		server.WithEngine(e),
		server.WithRouter(func(r fiber.Router) {
			r.Get("", func(c *fiber.Ctx) error {
				return c.SendString("Hello, World!")
			})
			r.Get("/owner/:name", func(c *fiber.Ctx) error {
				name := c.Params("name")
				if name == "" {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"error": "Missing name",
					})
				}
				question := &events.Question{
					Event: "opns:" + name,
					Spent: &engine.FALSE,
				}
				if outputs, err := lookupService.LookupOutputs(c.Context(), question); err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error": err.Error(),
					})
				} else if len(outputs) == 0 {
					return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
						"error": "No answer found",
					})
				} else if events, err := lookupService.FindEvents(c.Context(), &outputs[0].Outpoint); err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error": err.Error(),
					})
				} else {
					var address string
					for _, event := range events {
						if strings.HasPrefix(event, "p2pkh:") {
							address = strings.TrimPrefix(event, "p2pkh:")
						}
					}

					return c.JSON(fiber.Map{
						"address":  address,
						"outpoint": outputs[0].Outpoint.OrdinalString(),
					})
				}
			})

			r.Get("/mine/:name", func(c *fiber.Ctx) error {
				name := c.Params("name")
				if name == "" {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"error": "Missing name",
					})
				}
				question := &events.Question{
					Event: "mine:" + name,
				}
				if outputs, err := lookupService.LookupOutputs(c.Context(), question); err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error": err.Error(),
					})
				} else if len(outputs) == 0 {
					return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
						"error": "No answer found",
					})
				} else {
					return c.JSON(fiber.Map{
						"outpoint": outputs[0].Outpoint.OrdinalString(),
					})
				}
			})

			r.Post("/arc-ingest", func(c *fiber.Ctx) error {
				var status broadcaster.ArcResponse
				if err := c.BodyParser(&status); err != nil {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"error": "Invalid request",
					})
				} else if txid, err := chainhash.NewHashFromHex(status.Txid); err != nil {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"error": "Invalid txid",
					})
				} else if merklePath, err := transaction.NewMerklePathFromHex(status.MerklePath); err != nil {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"error": "Invalid merkle path",
					})
				} else if err := e.HandleNewMerkleProof(c.Context(), txid, merklePath); err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error": err.Error(),
					})
				} else {
					return c.JSON(fiber.Map{
						"status": "success",
					})
				}
			})

			r.Get("/subscribe/:topics", func(c *fiber.Ctx) error {
				topicsParam := c.Params("topics")
				if topicsParam == "" {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"error": "Missing topics",
					})
				}
				topics := strings.Split(topicsParam, ",")
				if len(topics) == 0 {
					return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
						"error": "No topics provided",
					})
				}

				// Set headers for SSE
				c.Set("Content-Type", "text/event-stream")
				c.Set("Cache-Control", "no-cache")
				c.Set("Connection", "keep-alive")

				// Add the client to the topicClients map
				writer := bufio.NewWriter(c.Context().Response.BodyWriter())
				subReq := &subRequest{
					topics: topics,
					writer: writer,
				}
				subscribe <- subReq

				// Wait for the client to disconnect
				<-c.Context().Done()
				unsubscribe <- subReq

				log.Println("Client disconnected:", topics)
				return nil
			})
		}),
		server.WithConfig(&appconfig.Config{
			Port: PORT,
		}),
	)

	// Start the Redis PubSub goroutine
	go func() {
		pubSub := sub.PSubscribe(ctx, "*")
		pubSubChan := pubSub.Channel() // Subscribe to all topics
		defer pubSub.Close()

		topicClients := make(map[string][]*bufio.Writer) // Map of topic to connected clients

		for {
			select {
			case <-ctx.Done():
				log.Println("Broadcasting stopped")
				return

			case msg := <-pubSubChan:
				// Broadcast the message to all clients subscribed to the topic
				if clients, exists := topicClients[msg.Channel]; exists {
					for _, client := range clients {
						parts := strings.Split(msg.Payload, ":")
						if len(parts) != 2 {
							log.Println("Invalid message format:", msg.Payload)
							continue
						}
						_, _ = fmt.Fprintf(client, "event: %s\n", msg.Channel)
						_, _ = fmt.Fprintf(client, "data: %s\n", parts[1])
						_, _ = fmt.Fprintf(client, "id: %s\n\n", parts[0])
						_ = client.Flush()
					}
				}

			case subReq := <-subscribe:
				// Add the client to the topicClients map
				for _, topic := range subReq.topics {
					topicClients[topic] = append(topicClients[topic], subReq.writer)
				}

			case subReq := <-unsubscribe:
				// Remove the client from the topicClients map
				for _, topic := range subReq.topics {
					clients := topicClients[topic]
					for i, client := range clients {
						if client == subReq.writer {
							topicClients[topic] = append(clients[:i], clients[i+1:]...)
							break
						}
					}
				}
			}
		}
	}()

	// Goroutine to handle OS signals
	go func() {
		<-signalChan
		log.Println("Shutting down server...")

		// Cancel the context to stop goroutines
		cancel()

		// Gracefully shut down the Fiber app
		// if err := app.Shutdown(); err != nil {
		// 	log.Fatalf("Error shutting down server: %v", err)
		// }

		// Close Redis connections
		if err := rdb.Close(); err != nil {
			log.Printf("Error closing Redis client: %v", err)
		}
		if err := sub.Close(); err != nil {
			log.Printf("Error closing Redis subscription client: %v", err)
		}

		log.Println("Server stopped.")
		os.Exit(0)
	}()

	// hack in paymail for now
	// go func() {
	// 	logger := logging.GetDefaultLogger()

	// 	sl := server.PaymailServiceLocator{}
	// 	sl.RegisterPaymailService(new(opnspaymail.OpnsServiceProvider))
	// 	sl.RegisterPikeContactService(new(opnspaymail.OpnsServiceProvider))
	// 	sl.RegisterPikePaymentService(new(opnspaymail.OpnsServiceProvider))

	// 	var err error
	// 	port := 3001
	// 	// portEnv := os.Getenv("PORT")
	// 	// if portEnv != "" {
	// 	// 	if port, err = strconv.Atoi(portEnv); err != nil {
	// 	// 		logger.Fatal().Msg(err.Error())
	// 	// 	}
	// 	// }
	// 	// Custom server with lots of customizable goodies
	// 	config, err := server.NewConfig(
	// 		&sl,
	// 		server.WithBasicRoutes(),
	// 		server.WithP2PCapabilities(),
	// 		server.WithBeefCapabilities(),
	// 		// server.WithDomain("1sat.app"),
	// 		server.WithDomain(os.Getenv("PAYMAIL_DOMAIN")),
	// 		// server.WithDomain("localhost:3000"),
	// 		// server.WithGenericCapabilities(),
	// 		server.WithPort(port),
	// 		// server.WithServiceName("BsvAliasCustom"),
	// 		server.WithTimeout(15*time.Second),
	// 		// server.WithCapabilities(customCapabilities()),
	// 	)
	// 	if err != nil {
	// 		logger.Fatal().Msg(err.Error())
	// 	}
	// 	config.Prefix = "https://" //normally paymail requires https, but for demo purposes we'll use http

	// 	// Create & start the server
	// 	server.StartServer(server.CreateServer(config), config.Logger)
	// }()

	if SYNC {
		if err := e.StartGASPSync(context.Background()); err != nil {
			log.Fatalf("Error starting sync: %v", err)
		}
	}
	// Start the server on the specified port
	<-http.StartWithGracefulShutdown(ctx)

}
