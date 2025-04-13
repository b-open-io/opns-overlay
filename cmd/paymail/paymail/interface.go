package paymail

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/b-open-io/opns-overlay/opns"
	"github.com/bitcoin-sv/go-paymail"
	"github.com/bitcoin-sv/go-paymail/server"
	"github.com/bitcoin-sv/go-paymail/spv"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
)

type OpnsServiceProvider struct {
	Lookup *opns.LookupService
}

type Opns struct {
	Outpoint string                 `json:"outpoint"`
	Origin   string                 `json:"origin"`
	Owner    string                 `json:"owner"`
	Domain   string                 `json:"domain"`
	Map      map[string]interface{} `json:"map,omitempty"`
}

type OwnerResult struct {
	Outpoint string `json:"outpoint"`
	Address  string `json:"address"`
}

var ErrNotFound = errors.New("not found")

func (p *OpnsServiceProvider) GetAddressStringByAlias(ctx context.Context, alias, domain string) (string, error) {
	if result, err := p.Lookup.Owner(ctx, alias); err != nil {
		return "", err
	} else if result == nil {
		return "", ErrNotFound
	} else {
		return result.Address, nil
	}
}

// GetPaymailByAlias is a demo implementation of this interface
func (d *OpnsServiceProvider) GetPaymailByAlias(ctx context.Context, alias, domain string,
	_ *server.RequestMetadata,
) (*paymail.AddressInformation, error) {
	if add, err := d.GetAddressStringByAlias(ctx, alias, domain); err != nil {
		return nil, err
	} else {
		return &paymail.AddressInformation{
			Alias:       alias,
			Domain:      domain,
			LastAddress: add,
			PubKey:      "000000000000000000000000000000000000000000000000000000000000000000",
		}, nil
	}
}

// CreateAddressResolutionResponse is a demo implementation of this interface
func (d *OpnsServiceProvider) CreateAddressResolutionResponse(ctx context.Context, alias, domain string,
	senderValidation bool, _ *server.RequestMetadata,
) (*paymail.ResolutionPayload, error) {
	// Generate a new destination / output for the basic address resolution
	if add, err := d.GetAddressStringByAlias(ctx, alias, domain); err != nil {
		return nil, err
	} else if address, err := script.NewAddressFromString(add); err != nil {
		return nil, err
	} else if lockingScript, err := p2pkh.Lock(address); err != nil {
		return nil, err
	} else {
		response := &paymail.ResolutionPayload{
			Output: hex.EncodeToString(*lockingScript),
		}
		// if senderValidation {
		// 	if response.Signature, err = bitcoin.SignMessage(
		// 		p.PrivateKey, response.Output, false,
		// 	); err != nil {
		// 		return nil, errors.New("invalid signature: " + err.Error())
		// 	}
		// }
		return response, nil
	}
}

// CreateP2PDestinationResponse is a demo implementation of this interface
func (d *OpnsServiceProvider) CreateP2PDestinationResponse(ctx context.Context, alias, domain string,
	satoshis uint64, _ *server.RequestMetadata,
) (*paymail.PaymentDestinationPayload, error) {
	// Generate a new destination for the p2p request
	output := &paymail.PaymentOutput{
		Satoshis: satoshis,
	}
	if add, err := d.GetAddressStringByAlias(ctx, alias, domain); err != nil {
		return nil, err
	} else if address, err := script.NewAddressFromString(add); err != nil {
		return nil, err
	} else if lockingScript, err := p2pkh.Lock(address); err != nil {
		return nil, err
	} else {
		output.Script = hex.EncodeToString(*lockingScript)
		// Create the response
		ref := make([]byte, 16)
		rand.Read(ref)
		d.Lookup.Db.Set(ctx, fmt.Sprintf("pay:%x", ref), alias, time.Minute).Err()
		return &paymail.PaymentDestinationPayload{
			Outputs:   []*paymail.PaymentOutput{output},
			Reference: hex.EncodeToString(ref),
		}, nil
	}
}

// RecordTransaction is a demo implementation of this interface
func (d *OpnsServiceProvider) RecordTransaction(ctx context.Context,
	p2pTx *paymail.P2PTransaction, _ *server.RequestMetadata,
) (payload *paymail.P2PTransactionPayload, err error) {
	var tx *transaction.Transaction
	payload = &paymail.P2PTransactionPayload{
		Note: "Transaction recorded",
	}
	if p2pTx.Beef != "" {
		if tx, err = transaction.NewTransactionFromBEEFHex(p2pTx.Beef); err != nil {
			return nil, err
		}
	} else if p2pTx.Hex != "" {
		if tx, err = transaction.NewTransactionFromHex(p2pTx.Hex); err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("no beef or hex provided")
	}
	payload.TxID = tx.TxID().String()
	return payload, nil
	// Record the tx into your datastore layer
}

// VerifyMerkleRoots is a demo implementation of this interface
func (d *OpnsServiceProvider) VerifyMerkleRoots(ctx context.Context, merkleProofs []*spv.MerkleRootConfirmationRequestItem) error {
	// Verify the Merkle roots
	return nil
}

func (d *OpnsServiceProvider) AddContact(
	ctx context.Context,
	requesterPaymail string,
	contact *paymail.PikeContactRequestPayload,
) error {
	return nil
}

func (d *OpnsServiceProvider) CreatePikeOutputResponse(
	ctx context.Context,
	alias, domain, senderPubKey string,
	satoshis uint64,
	metaData *server.RequestMetadata,
) (*paymail.PikePaymentOutputsResponse, error) {
	return nil, nil
}
