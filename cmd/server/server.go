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
	"time"

	"github.com/4chain-ag/go-overlay-services/pkg/core/engine"
	"github.com/4chain-ag/go-overlay-services/pkg/server"
	"github.com/b-open-io/opns-overlay/opns"
	"github.com/b-open-io/overlay/storage"
	"github.com/b-open-io/overlay/util"
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
	"github.com/valyala/fasthttp"
)

var chaintracker headers_client.Client
var PORT int
var SYNC bool
var rdb, sub *redis.Client
var peers = []string{}

type subRequest struct {
	topics  []string
	msgChan chan *redis.Message
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

	txStore, err := util.NewRedisTxStorage(os.Getenv("REDIS_BEEF"))
	if err != nil {
		log.Fatalf("Failed to initialize tx storage: %v", err)
	}
	store, err := storage.NewRedisStorage(os.Getenv("REDIS"), txStore)
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
		Storage:      store,
		ChainTracker: chaintracker,
		SyncConfiguration: map[string]engine.SyncConfiguration{
			tm: {
				Type:        engine.SyncConfigurationPeers,
				Peers:       peers,
				Concurrency: 1,
			},
		},
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
				} else if owner, err := lookupService.Owner(c.Context(), name); err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error": err.Error(),
					})
				} else if owner == nil {
					return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
						"error": "No owner found",
					})
				} else {
					return c.JSON(owner)
				}
			})

			r.Get("/mine/:name", func(c *fiber.Ctx) error {
				name := c.Params("name")
				if name == "" {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"error": "Missing name",
					})
				} else if outpoint, err := lookupService.Mine(c.Context(), name); err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error": err.Error(),
					})
				} else if outpoint == nil {
					return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
						"error": "No outpoint found",
					})
				} else {
					return c.JSON(fiber.Map{
						"outpoint": outpoint,
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
				c.Set("Transfer-Encoding", "chunked")

				// Add the client to the topicClients map
				// writer := c.Context().Response.BodyWriter()
				subReq := &subRequest{
					topics:  topics,
					msgChan: make(chan *redis.Message, 25),
				}
				subscribe <- subReq
				ctx := c.Context()
				c.Response().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
					// fmt.Println("WRITER")
					var i int
					// defer fmt.Println("CLOSING")
					for {
						select {
						case <-ctx.Done():
							unsubscribe <- subReq
							return
						case msg := <-subReq.msgChan:
							i++
							fmt.Fprintf(w, "id: %d\n", time.Now().UnixNano())
							fmt.Fprintf(w, "event: %s\n", msg.Channel)
							fmt.Fprintf(w, "data: %s\n\n", msg.Payload)
							err := w.Flush()
							if err != nil {
								// Refreshing page in web browser will establish a new
								// SSE connection, but only (the last) one is alive, so
								// dead connections must be closed here.
								// fmt.Printf("Error while flushing: %v. Closing http connection.\n", err)
								unsubscribe <- subReq
								return
							}
						}
					}
				}))

				log.Println("Client disconnected:", topics)
				return nil
			})
		}),
		server.WithConfig(&appconfig.Config{
			Port: PORT,
		}),
	)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Start the Redis PubSub goroutine
	go func() {
		pubSub := sub.PSubscribe(ctx, "*")
		pubSubChan := pubSub.Channel() // Subscribe to all topics
		defer pubSub.Close()

		topicChannels := make(map[string][]chan *redis.Message) // Map of topic to connected clients

		for {
			select {
			case <-ctx.Done():
				log.Println("Broadcasting stopped")
				return

			case msg := <-pubSubChan:
				// Broadcast the message to all clients subscribed to the topic
				if channels, exists := topicChannels[msg.Channel]; exists {
					for _, channel := range channels {
						channel <- msg
					}
				}

			case subReq := <-subscribe:
				// Add the client to the topicClients map
				for _, topic := range subReq.topics {
					topicChannels[topic] = append(topicChannels[topic], subReq.msgChan)
				}

			case subReq := <-unsubscribe:
				// Remove the client from the topicClients map
				for _, topic := range subReq.topics {
					channels := topicChannels[topic]
					for i, c := range channels {
						if c == subReq.msgChan {
							topicChannels[topic] = append(channels[:i], channels[i+1:]...)
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

	if SYNC {
		go func() {
			if err := e.StartGASPSync(context.Background()); err != nil {
				log.Fatalf("Error starting sync: %v", err)
			}
			// Todo: Subscribe to topics via SSE
		}()
	}
	// Start the server on the specified port
	<-http.StartWithGracefulShutdown(ctx)

}
