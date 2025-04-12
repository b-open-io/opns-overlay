package opns

import (
	"context"
	"errors"
	"log"

	"github.com/4chain-ag/go-overlay-services/pkg/core/engine"
	"github.com/bitcoin-sv/go-templates/template/opns"
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/transaction"
)

type TopicManager struct {
	Storage engine.Storage
	Topic   string
}

func (tm *TopicManager) IdentifyAdmissableOutputs(ctx context.Context, beefBytes []byte, previousCoins map[uint32][]byte) (admit overlay.AdmittanceInstructions, err error) {
	_, tx, txid, err := transaction.ParseBeef(beefBytes)
	if err != nil {
		return admit, err
	} else if tx == nil {
		return admit, errors.New("transaction is nil")
	} else if txid.Equal(opns.GENESIS.Txid) {
		admit.OutputsToAdmit = append(admit.OutputsToAdmit, 0)
		return
	} else if len(previousCoins) == 0 {
		return
	}
	txidStr := txid.String()
	log.Println("Identifying admissible outputs for txid:", txidStr)
	for vin, inputBeefBytes := range previousCoins {
		if tx.Inputs[vin].SourceTransaction, err = transaction.NewTransactionFromBEEF(inputBeefBytes); err != nil {
			return admit, err
		}
	}

	ancillaryTxids := make(map[string]struct{})
	for vin, _ := range previousCoins {
		if tx.Inputs[vin].SourceTransaction == nil {
			return admit, err
		}
		txin := tx.Inputs[vin]
		txout := tx.Inputs[vin].SourceTransaction.Outputs[txin.SourceTxOutIndex]
		if o := opns.Decode(txout.LockingScript); o != nil || txin.SourceTXID.Equal(opns.GENESIS.Txid) {
			admit.CoinsToRetain = append(admit.CoinsToRetain, vin)
			admit.OutputsToAdmit = []uint32{0, 1, 2}
		} else if txout.Satoshis == 1 {
			satsIn := uint64(0)
			missingInput := false
			for _, input := range tx.Inputs[:vin] {
				sourceTxOut := input.SourceTxOutput()
				if sourceTxOut == nil {
					missingInput = true
					break
				}
				satsIn += sourceTxOut.Satoshis
			}
			if missingInput {
				continue
			}
			satsOut := uint64(0)
			for vout, output := range tx.Outputs {
				if satsOut < satsIn {
					satsOut += output.Satoshis
					continue
				} else if satsOut == satsIn {
					if output.Satoshis == 0 {
						continue
					} else if output.Satoshis == 1 {
						for _, input := range tx.Inputs[:vin] {
							if _, ok := ancillaryTxids[input.SourceTXID.String()]; !ok {
								ancillaryTxids[input.SourceTXID.String()] = struct{}{}
								admit.AncillaryTxids = append(admit.AncillaryTxids, input.SourceTXID)
							}
						}
						admit.CoinsToRetain = append(admit.CoinsToRetain, vin)
						admit.OutputsToAdmit = append(admit.OutputsToAdmit, uint32(vout))
					}
				}
				break
			}
		}
	}
	return
}

func (tm *TopicManager) IdentifyNeededInputs(ctx context.Context, beefBytes []byte) (neededInputs []*overlay.Outpoint, err error) {
	return neededInputs, nil
}

func (tm *TopicManager) GetDocumentation() string {
	return "OpNS Topic Manager"
}

func (tm *TopicManager) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name: "OpNS",
	}
}
