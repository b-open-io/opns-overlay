package opns

import (
	"context"
	"errors"

	"github.com/bitcoin-sv/go-templates/template/opns"
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/transaction"
)

type TopicManager struct{}

func (tm *TopicManager) IdentifyAdmissableOutputs(ctx context.Context, beefBytes []byte, previousCoins []uint32) (admit overlay.AdmittanceInstructions, err error) {
	_, tx, txid, err := transaction.ParseBeef(beefBytes)
	if err != nil {
		return admit, err
	} else if tx == nil {
		return admit, errors.New("transaction is nil")
	}

	if txid.Equal(opns.GENESIS.Txid) {
		admit.OutputsToAdmit = append(admit.OutputsToAdmit, 0)
		return
	}
	if len(previousCoins) == 0 {
		return
	}

	ancillaryTxids := make(map[string]struct{})
	for vin := range previousCoins {
		sourceOutput := tx.Inputs[vin].SourceTxOutput()
		ancillaryTxids[tx.Inputs[vin].SourceTXID.String()] = struct{}{}
		if o := opns.Decode(sourceOutput.LockingScript); o != nil || tx.Inputs[vin].SourceTXID.Equal(opns.GENESIS.Txid) {
			admit.CoinsToRetain = previousCoins
			admit.OutputsToAdmit = []uint32{0, 1, 2}
			return
		} else if sourceOutput.Satoshis == 1 {
			satsIn := uint64(0)
			for _, input := range tx.Inputs[:vin] {
				satsIn += input.SourceTxOutput().Satoshis
				ancillaryTxids[input.SourceTXID.String()] = struct{}{}
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
						admit.CoinsToRetain = previousCoins
						admit.OutputsToAdmit = append(admit.OutputsToAdmit, uint32(vout))
					}
				}
				break
			}
		}
	}
	return
}

func (tm *TopicManager) IdentifyNeededInputs(ctx context.Context, beefBytes []byte) ([]*overlay.Outpoint, error) {
	return []*overlay.Outpoint{}, nil
}

func (tm *TopicManager) GetDocumentation() string {
	return "OpNS Topic Manager"
}

func (tm *TopicManager) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name: "OpNS",
	}
}
