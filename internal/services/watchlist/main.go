package watchlist

import (
	"context"
	"encoding/json"
	. "github.com/go-ozzo/ozzo-validation"
	"github.com/tokend/stellar-deposit-svc/internal/horizon/getters"
	"github.com/tokend/stellar-deposit-svc/internal/horizon/query"
	"gitlab.com/distributed_lab/logan/v3"
	"gitlab.com/distributed_lab/logan/v3/errors"
	"gitlab.com/distributed_lab/running"
	"gitlab.com/tokend/regources/generated"
	"sync"
	"time"
)

type StellarDetails struct {
	StellarDeposit     bool   `json:"stellar_deposit"`
	AssetType          string `json:"stellar_asset_type"`
	Code               string `json:"stellar_asset_code"`
	ExternalSystemType int32  `json:"external_system_type,string"`
}

func (s StellarDetails) Validate() error {
	return ValidateStruct(&s,
		Field(&s.StellarDeposit, Required),
		Field(&s.Code, Required, Length(1, 12)),
		Field(&s.AssetType, Required),
		Field(&s.ExternalSystemType, Required, Min(1)),
	)
}

type Details struct {
	regources.Asset
	StellarDetails
}

type Service struct {
	streamer getters.AssetHandler
	log      *logan.Entry
	owner    string
	timeout  time.Duration
	pipe     chan Details
	wg       *sync.WaitGroup
}

type Opts struct {
	Streamer   getters.AssetHandler
	Log        *logan.Entry
	AssetOwner string
	Timeout    time.Duration
	Wg         *sync.WaitGroup
}

func New(opts Opts) *Service {
	ch := make(chan Details)
	return &Service{
		streamer: opts.Streamer,
		owner:    opts.AssetOwner,
		log:      opts.Log.WithField("service", "watchlist"),
		pipe:     ch,
		timeout: opts.Timeout,
	}
}

func (s *Service) GetChan() <-chan Details {
	return s.pipe
}

func (s *Service) Run(ctx context.Context) {
	defer close(s.pipe)
	defer s.wg.Done()

	running.WithBackOff(ctx, s.log, "asset-watcher", func(ctx context.Context) error {
		assetsToWatch, err := s.getWatchList()
		if err != nil {
			return errors.Wrap(err, "failed to get asset watch list")
		}
		for _, asset := range assetsToWatch {
			s.pipe <- asset
		}
		return nil
	}, s.timeout, 20*time.Second, 5*time.Minute)
}

func (s *Service) getWatchList() ([]Details, error) {
	s.streamer.SetFilters(query.AssetFilters{Owner: &s.owner})

	assetsResponse, err := s.streamer.List()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get asset list for owner", logan.F{
			"owner_address": s.owner,
		})
	}

	watchList, err := s.filter(assetsResponse.Data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to filter asset list")
	}

	links := assetsResponse.Links
	for len(assetsResponse.Data) > 0 {
		assetsResponse, err = s.streamer.Next()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get next page of assetsResponse", logan.F{
				"links": links,
			})
		}

		links = assetsResponse.Links
		filtered, err := s.filter(assetsResponse.Data)
		if err != nil {
			return nil, errors.Wrap(err, "failed to filter asset list")
		}
		watchList = append(watchList, filtered...)
	}

	return watchList, nil
}

func (s *Service) filter(assets []regources.Asset) ([]Details, error) {
	result := make([]Details, 0, len(assets))
	for _, asset := range assets {
		details := asset.Attributes.Details
		stellarDetails := StellarDetails{}
		err := json.Unmarshal([]byte(details), &stellarDetails)
		if err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal asset details", logan.F{
				"asset_code":    asset.ID,
				"asset_details": details,
			})
		}
		if !stellarDetails.StellarDeposit {
			continue
		}

		if err = stellarDetails.Validate(); err != nil {
			return nil, errors.Wrap(err, "incorrect asset details")
		}

		if stellarDetails.StellarDeposit {
			result = append(result, Details{
				Asset:          asset,
				StellarDetails: stellarDetails,
			})
		}
	}

	return result, nil
}