package ticker

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/vazha/gocryptotrader/currency"
	"github.com/vazha/gocryptotrader/dispatch"
	"github.com/vazha/gocryptotrader/exchanges/asset"
)

func init() {
	service = new(Service)
	service.Tickers = make(map[string]map[*currency.Item]map[*currency.Item]map[asset.Item]*Ticker)
	service.Exchange = make(map[string]uuid.UUID)
	service.mux = dispatch.GetNewMux()
}

// SubscribeTicker subcribes to a ticker and returns a communication channel to
// stream new ticker updates
func SubscribeTicker(exchange string, p currency.Pair, a asset.Item) (dispatch.Pipe, error) {
	exchange = strings.ToLower(exchange)
	service.RLock()
	defer service.RUnlock()

	tick, ok := service.Tickers[exchange][p.Base.Item][p.Quote.Item][a]
	if !ok {
		return dispatch.Pipe{}, fmt.Errorf("ticker item not found for %s %s %s",
			exchange,
			p,
			a)
	}

	return service.mux.Subscribe(tick.Main)
}

// SubscribeToExchangeTickers subcribes to all tickers on an exchange
func SubscribeToExchangeTickers(exchange string) (dispatch.Pipe, error) {
	exchange = strings.ToLower(exchange)
	service.RLock()
	defer service.RUnlock()
	id, ok := service.Exchange[exchange]
	if !ok {
		return dispatch.Pipe{}, fmt.Errorf("%s exchange tickers not found",
			exchange)
	}

	return service.mux.Subscribe(id)
}

// GetTicker checks and returns a requested ticker if it exists
func GetTicker(exchange string, p currency.Pair, tickerType asset.Item) (*Price, error) {
	exchange = strings.ToLower(exchange)
	service.RLock()
	defer service.RUnlock()
	if service.Tickers[exchange] == nil {
		return nil, fmt.Errorf("no tickers for %s exchange", exchange)
	}

	if service.Tickers[exchange][p.Base.Item] == nil {
		return nil, fmt.Errorf("no tickers associated with base currency %s",
			p.Base)
	}

	if service.Tickers[exchange][p.Base.Item][p.Quote.Item] == nil {
		return nil, fmt.Errorf("no tickers associated with quote currency %s",
			p.Quote)
	}

	if service.Tickers[exchange][p.Base.Item][p.Quote.Item][tickerType] == nil {
		return nil, fmt.Errorf("no tickers associated with asset type %s",
			tickerType)
	}

	return &service.Tickers[exchange][p.Base.Item][p.Quote.Item][tickerType].Price, nil
}

// ProcessTicker processes incoming tickers, creating or updating the Tickers
// list
func ProcessTicker(exchangeName string, tickerNew *Price, assetType asset.Item) error {
	if exchangeName == "" {
		return fmt.Errorf(errExchangeNameUnset)
	}

	tickerNew.ExchangeName = strings.ToLower(exchangeName)

	if tickerNew.Pair.IsEmpty() {
		return fmt.Errorf("%s %s", exchangeName, errPairNotSet)
	}

	if assetType == "" {
		return fmt.Errorf("%s %s %s", exchangeName,
			tickerNew.Pair,
			errAssetTypeNotSet)
	}

	tickerNew.AssetType = assetType

	if tickerNew.LastUpdated.IsZero() {
		tickerNew.LastUpdated = time.Now()
	}

	return service.Update(tickerNew)
}

// Update updates ticker price
func (s *Service) Update(p *Price) error {
	var ids []uuid.UUID

	s.Lock()
	switch {
	case s.Tickers[p.ExchangeName] == nil:
		s.Tickers[p.ExchangeName] = make(map[*currency.Item]map[*currency.Item]map[asset.Item]*Ticker)
		s.Tickers[p.ExchangeName][p.Pair.Base.Item] = make(map[*currency.Item]map[asset.Item]*Ticker)
		s.Tickers[p.ExchangeName][p.Pair.Base.Item][p.Pair.Quote.Item] = make(map[asset.Item]*Ticker)
		err := s.SetItemID(p)
		if err != nil {
			s.Unlock()
			return err
		}

	case s.Tickers[p.ExchangeName][p.Pair.Base.Item] == nil:
		s.Tickers[p.ExchangeName][p.Pair.Base.Item] = make(map[*currency.Item]map[asset.Item]*Ticker)
		s.Tickers[p.ExchangeName][p.Pair.Base.Item][p.Pair.Quote.Item] = make(map[asset.Item]*Ticker)
		err := s.SetItemID(p)
		if err != nil {
			s.Unlock()
			return err
		}

	case s.Tickers[p.ExchangeName][p.Pair.Base.Item][p.Pair.Quote.Item] == nil:
		s.Tickers[p.ExchangeName][p.Pair.Base.Item][p.Pair.Quote.Item] = make(map[asset.Item]*Ticker)
		err := s.SetItemID(p)
		if err != nil {
			s.Unlock()
			return err
		}

	case s.Tickers[p.ExchangeName][p.Pair.Base.Item][p.Pair.Quote.Item][p.AssetType] == nil:
		err := s.SetItemID(p)
		if err != nil {
			s.Unlock()
			return err
		}

	default:
		ticker := s.Tickers[p.ExchangeName][p.Pair.Base.Item][p.Pair.Quote.Item][p.AssetType]
		ticker.Last = p.Last
		ticker.High = p.High
		ticker.Low = p.Low
		ticker.Bid = p.Bid
		ticker.Ask = p.Ask
		ticker.Volume = p.Volume
		ticker.QuoteVolume = p.QuoteVolume
		ticker.PriceATH = p.PriceATH
		ticker.Open = p.Open
		ticker.Close = p.Close
		ticker.LastUpdated = p.LastUpdated
		ids = ticker.Assoc
		ids = append(ids, ticker.Main)
	}
	s.Unlock()
	return s.mux.Publish(ids, p)
}

// SetItemID retrieves and sets dispatch mux publish IDs
func (s *Service) SetItemID(p *Price) error {
	if p == nil {
		return errors.New(errTickerPriceIsNil)
	}

	ids, err := s.GetAssociations(p)
	if err != nil {
		return err
	}
	singleID, err := s.mux.GetID()
	if err != nil {
		return err
	}

	s.Tickers[p.ExchangeName][p.Pair.Base.Item][p.Pair.Quote.Item][p.AssetType] = &Ticker{Price: *p,
		Main:  singleID,
		Assoc: ids}
	return nil
}

// GetAssociations links a singular book with it's dispatch associations
func (s *Service) GetAssociations(p *Price) ([]uuid.UUID, error) {
	if p == nil || *p == (Price{}) {
		return nil, errors.New(errTickerPriceIsNil)
	}
	var ids []uuid.UUID
	exchangeID, ok := s.Exchange[p.ExchangeName]
	if !ok {
		var err error
		exchangeID, err = s.mux.GetID()
		if err != nil {
			return nil, err
		}
		s.Exchange[p.ExchangeName] = exchangeID
	}

	ids = append(ids, exchangeID)
	return ids, nil
}
