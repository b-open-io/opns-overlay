package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/4chain-ag/go-overlay-services/pkg/core/engine"
	"github.com/GorillaPool/go-junglebus"
	"github.com/b-open-io/opns-overlay/opns"
	"github.com/b-open-io/overlay/storage"
	"github.com/b-open-io/overlay/util"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/chaintracker/headers_client"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

var JUNGLEBUS = "https://texas1.junglebus.gorillapool.io"
var jb *junglebus.Client
var chaintracker headers_client.Client

type tokenSummary struct {
	tx   int
	out  int
	time time.Duration
}

func init() {
	godotenv.Load("../../.env")
	jb, _ = junglebus.New(
		junglebus.WithHTTP(JUNGLEBUS),
	)
	chaintracker = headers_client.Client{
		Url:    os.Getenv("BLOCK_HEADERS_URL"),
		ApiKey: os.Getenv("BLOCK_HEADERS_API_KEY"),
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals for graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChan
		log.Println("Received shutdown signal, cleaning up...")
		cancel()
	}()

	var rdb *redis.Client
	log.Println("Connecting to Redis", os.Getenv("REDIS"))
	if opts, err := redis.ParseURL(os.Getenv("REDIS")); err != nil {
		log.Fatalf("Failed to parse Redis URL: %v", err)
	} else {
		rdb = redis.NewClient(opts)
	}
	// Initialize storage
	txStore, err := util.NewRedisTxStorage(os.Getenv("REDIS_BEEF"))
	if err != nil {
		log.Fatalf("Failed to initialize tx storage: %v", err)
	}
	storage, err := storage.NewRedisStorage(os.Getenv("REDIS"), txStore)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer storage.Close()

	lookupService, err := opns.NewLookupService(
		os.Getenv("REDIS"),
		storage,
		"tm_OpNS",
	)
	if err != nil {
		log.Fatalf("Failed to initialize lookup service: %v", err)
	}
	tm := "tm_OpNS"
	e := engine.Engine{
		Managers: map[string]engine.TopicManager{
			tm: &opns.TopicManager{},
		},
		LookupServices: map[string]engine.LookupService{
			"ls_OpNS": lookupService,
		},
		Storage:      storage,
		ChainTracker: chaintracker,
		PanicOnError: true,
	}

	done := make(chan *tokenSummary, 1000)
	go func() {
		ticker := time.NewTicker(time.Minute)
		txcount := 0
		outcount := 0
		// accTime
		lastTime := time.Now()
		for {
			select {
			case summary := <-done:
				txcount += summary.tx
				outcount += summary.out
				// log.Println("Got done")

			case <-ticker.C:
				log.Printf("Processed tx %d o %d in %v %vtx/s\n", txcount, outcount, time.Since(lastTime), float64(txcount)/time.Since(lastTime).Seconds())
				lastTime = time.Now()
				txcount = 0
				outcount = 0
			case <-ctx.Done():
				log.Println("Context canceled, stopping processing...")
				return
			}
		}
	}()

	txids, err := rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     "opns",
		Stop:    "+inf",
		Start:   "-inf",
		ByScore: true,
	}).Result()
	if err != nil {
		log.Fatalf("Failed to query Redis: %v", err)
	}

	txids = append([]string{"58b7558ea379f24266c7e2f5fe321992ad9a724fd7a87423ba412677179ccb25"}, txids...)

	for _, txidStr := range txids {
		select {
		case <-ctx.Done():
			log.Println("Context canceled, stopping processing...")
			return
		default:
			if txid, err := chainhash.NewHashFromHex(txidStr); err != nil {
				log.Fatalf("Invalid txid: %v", err)
			} else if tx, err := txStore.LoadTx(ctx, txid); err != nil {
				log.Fatalf("Failed to load transaction: %v", err)
			} else {
				beef := &transaction.Beef{
					Version:      transaction.BEEF_V2,
					Transactions: map[string]*transaction.BeefTx{},
				}
				for _, input := range tx.Inputs {
					if input.SourceTransaction, err = txStore.LoadTx(ctx, input.SourceTXID); err != nil {
						log.Fatalf("Failed to load source transaction: %v", err)
					} else if _, err := beef.MergeTransaction(input.SourceTransaction); err != nil {
						log.Fatalf("Failed to merge source transaction: %v", err)
					}
				}
				if _, err := beef.MergeTransaction(tx); err != nil {
					log.Fatalf("Failed to merge source transaction: %v", err)
				}

				taggedBeef := overlay.TaggedBEEF{
					Topics: []string{tm},
				}
				// log.Println("Tx Loaded", tx.TxID().String(), "in", time.Since(start))
				logTime := time.Now()
				if taggedBeef.Beef, err = beef.AtomicBytes(txid); err != nil {
					log.Fatalf("Failed to generate BEEF: %v", err)
				} else if admit, err := e.Submit(ctx, taggedBeef, engine.SubmitModeHistorical, nil); err != nil {
					log.Fatalf("Failed to submit transaction: %v", err)
				} else {
					// log.Println("Submitted generated", tx.TxID().String(), "in", time.Since(logTime))
					// logTime = time.Now()
					if err := rdb.ZRem(ctx, "opns", txidStr).Err(); err != nil {
						log.Fatalf("Failed to delete from queue: %v", err)
					}
					log.Println("Processed", txid, "in", time.Since(logTime), "as", admit[tm].OutputsToAdmit)
					done <- &tokenSummary{
						tx:  1,
						out: len(admit[tm].OutputsToAdmit),
					}
					// start = time.Now()
				}
			}
		}
	}

	// Close the database connection
	// sub.QueueDb.Close()
	log.Println("Application shutdown complete.")
}
