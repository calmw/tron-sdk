package transaction

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/calmw/tron-sdk/pkg/client"
	"github.com/calmw/tron-sdk/pkg/common"
	"github.com/calmw/tron-sdk/pkg/keystore"
	"github.com/calmw/tron-sdk/pkg/ledger"
	"github.com/calmw/tron-sdk/pkg/proto/api"
	"github.com/calmw/tron-sdk/pkg/proto/core"
	proto "google.golang.org/protobuf/proto"
)

var (
	// ErrBadTransactionParam is returned when invalid params are given to the
	// controller upon execution of a transaction.
	ErrBadTransactionParam = errors.New("transaction has bad parameters")
)

type sender struct {
	Ks      *keystore.KeyStore
	Account *keystore.Account
}

// Controller drives the transaction signing process
type Controller struct {
	ExecutionError error
	resultError    error
	Client         *client.GrpcClient
	Tx             *core.Transaction
	Sender         sender
	Behavior       behavior
	Result         *api.Return
	Receipt        *core.TransactionInfo
}

type behavior struct {
	DryRun               bool
	SigningImpl          SignerImpl
	ConfirmationWaitTime uint32
}

// NewController initializes a Controller, caller can control behavior via options
func NewController(
	client *client.GrpcClient,
	senderKs *keystore.KeyStore,
	senderAcct *keystore.Account,
	tx *core.Transaction,
	options ...func(*Controller),
) *Controller {

	ctrlr := &Controller{
		ExecutionError: nil,
		resultError:    nil,
		Client:         client,
		Sender: sender{
			Ks:      senderKs,
			Account: senderAcct,
		},
		Tx:       tx,
		Behavior: behavior{false, Software, 0},
	}
	for _, option := range options {
		option(ctrlr)
	}
	return ctrlr
}

func (C *Controller) SignTxForSending() {
	if C.ExecutionError != nil {
		return
	}
	signedTransaction, err :=
		C.Sender.Ks.SignTx(*C.Sender.Account, C.Tx)
	if err != nil {
		C.ExecutionError = err
		return
	}
	C.Tx = signedTransaction
}

func (C *Controller) HardwareSignTxForSending() {
	if C.ExecutionError != nil {
		return
	}
	data, _ := C.GetRawData()
	signature, err := ledger.SignTx(data)
	if err != nil {
		C.ExecutionError = err
		return
	}

	/* TODO: validate signature
	if strings.Compare(signerAddr, address.ToBech32(C.Sender.Account.Address)) != 0 {
		C.ExecutionError = ErrBadTransactionParam
		errorMsg := "signature verification failed : Sender address doesn't match with ledger hardware address"
		C.transactionErrors = append(C.transactionErrors, &Error{
			ErrMessage:           &errorMsg,
			TimestampOfRejection: time.Now().Unix(),
		})
		return
	}
	*/
	// add signature
	C.Tx.Signature = append(C.Tx.Signature, signature)
}

// TransactionHash extract hash from TX
func (C *Controller) TransactionHash() (string, error) {
	rawData, err := C.GetRawData()
	if err != nil {
		return "", err
	}
	h256h := sha256.New()
	h256h.Write(rawData)
	hash := h256h.Sum(nil)
	return common.ToHex(hash), nil
}

func (C *Controller) TxConfirmation() {
	if C.ExecutionError != nil || C.Behavior.DryRun {
		return
	}
	if C.Behavior.ConfirmationWaitTime > 0 {
		txHash, err := C.TransactionHash()
		if err != nil {
			C.ExecutionError = fmt.Errorf("could not get Tx hash")
			return
		}
		//fmt.Printf("TX hash: %s\nWaiting for confirmation....", txHash)
		start := int(C.Behavior.ConfirmationWaitTime)
		for {
			// GETTX by ID
			if txi, err := C.Client.GetTransactionInfoByID(txHash); err == nil {
				// check receipt
				if txi.Result != 0 {
					C.resultError = fmt.Errorf("%s", txi.ResMessage)
				}
				// Add receipt
				C.Receipt = txi
				return
			}
			if start < 0 {
				C.ExecutionError = fmt.Errorf("could not confirm transaction after %d seconds", C.Behavior.ConfirmationWaitTime)
				return
			}
			time.Sleep(time.Second)
			start--
		}
	} else {
		C.Receipt = &core.TransactionInfo{}
		C.Receipt.Receipt = &core.ResourceReceipt{}
	}

}

// GetResultError return result error
func (C *Controller) GetResultError() error {
	return C.resultError
}

// ExecuteTransaction is the single entrypoint to execute a plain transaction.
// Each step in transaction creation, execution probably includes a mutation
// Each becomes a no-op if ExecutionError occurred in any previous step
func (C *Controller) ExecuteTransaction() error {
	switch C.Behavior.SigningImpl {
	case Software:
		C.SignTxForSending()
	case Ledger:
		C.HardwareSignTxForSending()
	}
	C.SendSignedTx()
	C.TxConfirmation()
	return C.ExecutionError
}

// GetRawData Byes from Transaction
func (C *Controller) GetRawData() ([]byte, error) {
	return proto.Marshal(C.Tx.GetRawData())
}

func (C *Controller) SendSignedTx() {
	if C.ExecutionError != nil || C.Behavior.DryRun {
		return
	}
	result, err := C.Client.Broadcast(C.Tx)
	if err != nil {
		C.ExecutionError = err
		return
	}
	if result.Code != 0 {
		C.ExecutionError = fmt.Errorf("bad transaction: %v", string(result.GetMessage()))
	}
	C.Result = result
}
