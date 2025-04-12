package opns

import (
	"context"
	"fmt"
	"strings"

	"github.com/4chain-ag/go-overlay-services/pkg/core/engine"
	"github.com/b-open-io/overlay/lookup/events"
	"github.com/bitcoin-sv/go-templates/template/inscription"
	"github.com/bitcoin-sv/go-templates/template/opns"
	"github.com/bitcoin-sv/go-templates/template/ordlock"
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
)

type LookupService struct {
	*events.RedisEventLookup
}

func NewLookupService(connString string, storage engine.Storage, topic string) (*LookupService, error) {
	if eventsLookup, err := events.NewRedisEventLookup(connString, storage, topic); err != nil {
		return nil, err
	} else {
		return &LookupService{
			eventsLookup,
		}, nil
	}
}

func (l *LookupService) OutputAdded(ctx context.Context, outpoint *overlay.Outpoint, topic string, beef []byte) error {
	outputEvents := make([]string, 0, 5)
	var domain string
	_, tx, _, err := transaction.ParseBeef(beef)
	if err != nil {
		return err
	}
	txOut := tx.Outputs[outpoint.OutputIndex]
	if txOut.Satoshis == 1 {
		satsOut := uint64(0)
		for _, output := range tx.Outputs[:outpoint.OutputIndex] {
			satsOut += output.Satoshis
		}
		satsIn := uint64(0)
		for _, input := range tx.Inputs {
			sourceOut := input.SourceTxOutput()
			if sourceOut == nil {
				break
			}
			if satsIn < satsOut {
				satsIn += sourceOut.Satoshis
				continue
			} else if satsIn == satsOut && sourceOut.Satoshis == 1 {
				outpoint := &overlay.Outpoint{
					Txid:        *input.SourceTXID,
					OutputIndex: input.SourceTxOutIndex,
				}
				if inputEvents, err := l.Db.SMembers(ctx, events.OutpointEventsKey(outpoint)).Result(); err != nil {
					return err
				} else {
					for _, event := range inputEvents {
						if strings.HasPrefix(event, "opns:") {
							domain = strings.TrimPrefix(event, "opns:")
							outputEvents = append(outputEvents, event)
						} else if strings.HasPrefix(event, "origin:") {
							outputEvents = append(outputEvents, event)
						}
					}
					break
				}
			} else {
				outputEvents = append(outputEvents, "origin:"+outpoint.OrdinalString())
			}
		}
	}
	if o := opns.Decode(txOut.LockingScript); o != nil {
		outputEvents = append(outputEvents, "mine:"+o.Domain)
	} else if insc := inscription.Decode(txOut.LockingScript); insc != nil && insc.File.Type == "application/op-ns" {
		domain = string(insc.File.Content)
		outputEvents = append(outputEvents, "opns:"+domain)
		if p := p2pkh.Decode(script.NewFromBytes(insc.ScriptPrefix), true); p != nil {
			outputEvents = append(outputEvents, fmt.Sprintf("p2pkh:%s", p.AddressString))
		} else if p := p2pkh.Decode(script.NewFromBytes(insc.ScriptSuffix), true); p != nil {
			outputEvents = append(outputEvents, fmt.Sprintf("p2pkh:%s", p.AddressString))
		}
	}
	if p := p2pkh.Decode(txOut.LockingScript, true); p != nil {
		outputEvents = append(outputEvents, fmt.Sprintf("p2pkh:%s", p.AddressString))
	} else if ol := ordlock.Decode(txOut.LockingScript); ol != nil && domain != "" {
		outputEvents = append(outputEvents, fmt.Sprintf("list:%s", domain))
	}
	var blockHeight uint32
	var blockIdx uint64
	if tx.MerklePath != nil {
		blockHeight = tx.MerklePath.BlockHeight
		for _, leaf := range tx.MerklePath.Path[0] {
			if leaf.Hash != nil && leaf.Hash.Equal(outpoint.Txid) {
				blockIdx = leaf.Offset
				break
			}
		}
	}
	l.SaveEvents(ctx, outpoint, outputEvents, blockHeight, blockIdx)
	return nil
}

func (l *LookupService) GetDocumentation() string {
	return "Events lookup"
}

func (l *LookupService) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name: "Events",
	}
}

type OwnerResult struct {
	Outpoint *overlay.Outpoint `json:"outpoint"`
	Address  string            `json:"address"`
}

func (l *LookupService) Owner(ctx context.Context, domain string) (*OwnerResult, error) {
	question := &events.Question{
		Event: "opns:" + domain,
		Spent: &engine.FALSE,
	}
	if outpoints, err := l.LookupOutpoints(ctx, question); err != nil {
		return nil, err
	} else if len(outpoints) == 0 {
		return nil, nil
	} else if len(outpoints) > 1 {
		return nil, fmt.Errorf("multiple outputs found for domain %s", domain)
	} else if evts, err := l.FindEvents(ctx, outpoints[0]); err != nil {
		return nil, err
	} else {
		for _, event := range evts {
			if strings.HasPrefix(event, "p2pkh:") {
				return &OwnerResult{
					Outpoint: outpoints[0],
					Address:  strings.TrimPrefix(event, "p2pkh:"),
				}, nil
			}
		}
		return nil, nil
	}
}

type MineResult struct {
	Outpoint *overlay.Outpoint `json:"outpoint"`
	Domain   string            `json:"domain"`
}

func (l *LookupService) Mine(ctx context.Context, domain string) (*MineResult, error) {
	question := &events.Question{
		Event: "mine:" + domain,
		Spent: &engine.FALSE,
	}
	if outpoints, err := l.LookupOutpoints(ctx, question); err != nil {
		return nil, err
	} else if len(outpoints) > 0 {
		return nil, nil
	}

	for len(question.Event) > 5 {
		question.Event = question.Event[:len(question.Event)-1]
		if outpoints, err := l.LookupOutpoints(ctx, question); err != nil {
			return nil, err
		} else if len(outpoints) > 1 {
			return nil, fmt.Errorf("multiple outputs found for domain %s", domain)
		} else if len(outpoints) == 1 {
			return &MineResult{
				Outpoint: outpoints[0],
				Domain:   strings.TrimPrefix(question.Event, "mine:"),
			}, nil
		}
	}
	return nil, nil
}
