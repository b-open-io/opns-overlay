package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
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
	sdkUtil "github.com/bsv-blockchain/go-sdk/util"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

var JUNGLEBUS = "https://texas1.junglebus.gorillapool.io"
var jb *junglebus.Client
var chaintracker headers_client.Client
var SPENDS = false

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
	flag.BoolVar(&SPENDS, "s", false, "Start sync")
	flag.Parse()

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
	// Initialize store
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
	e := engine.Engine{
		Managers: map[string]engine.TopicManager{
			tm: &opns.TopicManager{},
		},
		LookupServices: map[string]engine.LookupService{
			"ls_OpNS": lookupService,
		},
		Storage:      store,
		ChainTracker: chaintracker,
		PanicOnError: true,
	}

	limiter := make(chan struct{}, 128)
	done := make(chan *tokenSummary, 1000)
	queue := make(chan *chainhash.Hash, 1000)

	go func() {
		if SPENDS {
			log.Println("Syncing spends...")
			var wg sync.WaitGroup
			if outpoints, err := rdb.ZRangeByScore(ctx, storage.OutMembershipKey(tm), &redis.ZRangeBy{
				Min: "0",
				Max: "inf",
			}).Result(); err != nil {
				log.Fatalf("Failed to query Redis: %v", err)
			} else {
				wg.Add(len(outpoints))
				log.Println()
				for _, op := range outpoints {
					limiter <- struct{}{}
					go func(op string) {
						defer func() { <-limiter }()
						defer wg.Done()
						if outpoint, err := overlay.NewOutpointFromString(op); err != nil {
							log.Fatalf("Invalid outpoint: %v", err)
						} else if isSpent, err := rdb.HGet(ctx, storage.OutputTopicKey(outpoint, tm), "sp").Bool(); err != nil {
							log.Fatalf("Failed to get spent status: %v", err)
						} else if !isSpent {
							log.Println("Checking spend for", outpoint)
							if spend, err := lookupSpend(&overlay.Outpoint{
								Txid:        outpoint.Txid,
								OutputIndex: outpoint.OutputIndex,
							}); err != nil {
								log.Fatalf("Failed to lookup spend: %v", err)
							} else if spend != nil {
								log.Println("Found spend. Queing", spend)
								if err := rdb.ZAdd(ctx, "opns", redis.Z{
									Score:  float64(time.Now().UnixNano()),
									Member: spend.String(),
								}).Err(); err != nil {
									log.Fatalf("Failed to add spend to queue: %v", err)
								}
							}
						}
					}(op)
				}
			}
			wg.Wait()
			log.Println("Finished syncing spends")
		}
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
			if txid, err := chainhash.NewHashFromHex(txidStr); err != nil {
				log.Fatalf("Invalid txid: %v", err)
			} else {
				queue <- txid
			}
		}
	}()

	ticker := time.NewTicker(time.Minute / 6)
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

		case txid := <-queue:
			limiter <- struct{}{}
			go func(txid *chainhash.Hash) {
				defer func() { <-limiter }()
				txidStr := txid.String()
				// log.Println("Processing", txidStr)
				if txid, err := chainhash.NewHashFromHex(txidStr); err != nil {
					log.Fatalf("Invalid txid: %v", err)
				} else if tx, err := util.LoadTx(ctx, txid); err != nil {
					log.Fatalf("Failed to load transaction: %v", err)
				} else {
					logTime := time.Now()
					beef := &transaction.Beef{
						Version:      transaction.BEEF_V2,
						Transactions: map[string]*transaction.BeefTx{},
					}
					for _, input := range tx.Inputs {
						if input.SourceTransaction, err = util.LoadTx(ctx, input.SourceTXID); err != nil {
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

					if taggedBeef.Beef, err = beef.AtomicBytes(txid); err != nil {
						log.Fatalf("Failed to generate BEEF: %v", err)
					} else if admit, err := e.Submit(ctx, taggedBeef, engine.SubmitModeHistorical, nil); err != nil {
						log.Fatalf("Failed to submit transaction: %v", err)
					} else {
						for _, vout := range admit[tm].OutputsToAdmit {
							if spend, err := lookupSpend(&overlay.Outpoint{
								Txid:        *txid,
								OutputIndex: vout,
							}); err != nil {
								log.Fatalf("Failed to lookup spend: %v", err)
							} else if spend != nil {
								if err := rdb.ZAdd(ctx, "opns", redis.Z{
									Score:  float64(time.Now().UnixNano()),
									Member: spend.String(),
								}).Err(); err != nil {
									log.Fatalf("Failed to add spend to queue: %v", err)
								}
								queue <- spend
							}
						}
						if err := rdb.ZRem(ctx, "opns", txidStr).Err(); err != nil {
							log.Fatalf("Failed to delete from queue: %v", err)
						}
						log.Println("Processed", txid, "in", time.Since(logTime), "as", admit[tm].OutputsToAdmit)
						done <- &tokenSummary{
							tx:  1,
							out: len(admit[tm].OutputsToAdmit),
						}
					}
				}
			}(txid)
		}
	}

	// Close the database connection
	// sub.QueueDb.Close()

}

func lookupSpend(outpoint *overlay.Outpoint) (*chainhash.Hash, error) {
	if resp, err := http.Get(fmt.Sprintf("%s/v1/txo/spend/%s", os.Getenv("JUNGLEBUS"), outpoint.OrdinalString())); err != nil {
		return nil, err
	} else {
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("%d-%s", resp.StatusCode, outpoint)
		} else if spendBytes, err := io.ReadAll(resp.Body); err != nil {
			return nil, err
		} else if len(spendBytes) == 0 {
			return nil, nil
		} else {
			return chainhash.NewHash(sdkUtil.ReverseBytes(spendBytes))
		}
	}
}
