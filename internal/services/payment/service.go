package payment

import (
	"context"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/protocols/horizon/operations"
	"gitlab.com/distributed_lab/logan/v3"
	"gitlab.com/distributed_lab/logan/v3/errors"
	"gitlab.com/distributed_lab/running"
	"time"
)

func (s *Service) GetChan() <-chan Details {
	return s.ch
}

func (s *Service) Run(ctx context.Context) {
	request := horizonclient.OperationRequest{
		ForAccount: s.watchAddress,
		Order:      horizonclient.OrderAsc,
	}
	page, err := s.client.Operations(request)
	if err != nil {
		s.log.WithError(err).Error("failed to get payments page")
		return
	}

	running.WithBackOff(ctx, s.log, "stellar-payment-listener", func(ctx context.Context) error {
		err = s.processPaymentPage(page)
		if err != nil {
			return errors.Wrap(err, "failed to process payment page")
		}

		page, err = s.client.NextOperationsPage(page)
		if err != nil {
			return errors.Wrap(err, "failed to get next page of payments")
		}

		return nil
	}, 30*time.Second, 30*time.Second, time.Hour)
}

func paymentDetails(record operations.Payment, tx horizon.Transaction) Details {
	return Details{
		Payment: record,
		TxHash:  tx.Hash,
		TxMemo:  tx.Memo,
	}
}

func (s *Service) processPaymentPage(page operations.OperationsPage) error {
	for _, record := range page.Embedded.Records {
		payment, ok := record.(operations.Payment)
		if !ok {
			continue
		}

		if payment.Asset.Type != string(s.assetType) {
			continue
		}

		if payment.Asset.Type != string(horizonclient.AssetTypeNative) &&
			payment.Asset.Code != s.assetCode {
			continue
		}

		tx, err := s.client.TransactionDetail(record.GetTransactionHash())
		if err != nil {
			return errors.Wrap(err, "failed to get parent transaction of payment", logan.F{
				"tx_hash":    record.GetTransactionHash(),
				"payment_id": record.GetID(),
			})
		}
		s.ch <- paymentDetails(payment, tx)
	}

	return nil
}