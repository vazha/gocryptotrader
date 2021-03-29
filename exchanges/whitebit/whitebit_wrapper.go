package whitebit

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/thrasher-corp/gocryptotrader/common"
	"github.com/thrasher-corp/gocryptotrader/config"
	"github.com/thrasher-corp/gocryptotrader/currency"
	exchange "github.com/thrasher-corp/gocryptotrader/exchanges"
	"github.com/thrasher-corp/gocryptotrader/exchanges/account"
	"github.com/thrasher-corp/gocryptotrader/exchanges/asset"
	"github.com/thrasher-corp/gocryptotrader/exchanges/kline"
	"github.com/thrasher-corp/gocryptotrader/exchanges/order"
	"github.com/thrasher-corp/gocryptotrader/exchanges/orderbook"
	"github.com/thrasher-corp/gocryptotrader/exchanges/protocol"
	"github.com/thrasher-corp/gocryptotrader/exchanges/request"
	"github.com/thrasher-corp/gocryptotrader/exchanges/stream"
	"github.com/thrasher-corp/gocryptotrader/exchanges/ticker"
	"github.com/thrasher-corp/gocryptotrader/exchanges/trade"
	"github.com/thrasher-corp/gocryptotrader/log"
	"github.com/thrasher-corp/gocryptotrader/portfolio/withdraw"
)

// GetDefaultConfig returns a default exchange config
func (b *Whitebit) GetDefaultConfig() (*config.ExchangeConfig, error) {
	b.SetDefaults()
	exchCfg := new(config.ExchangeConfig)
	exchCfg.Name = b.Name
	exchCfg.HTTPTimeout = exchange.DefaultHTTPTimeout
	exchCfg.BaseCurrencies = b.BaseCurrencies

	err := b.SetupDefaults(exchCfg)
	if err != nil {
		return nil, err
	}

	if b.Features.Supports.RESTCapabilities.AutoPairUpdates {
		err = b.UpdateTradablePairs(true)
		if err != nil {
			return nil, err
		}
	}

	return exchCfg, nil
}

// SetDefaults sets the basic defaults for Whitebit
func (b *Whitebit) SetDefaults() {
	b.Name = "Whitebit"
	b.Enabled = true
	b.Verbose = true
	b.WebsocketSubdChannels = make(map[int]WebsocketChanInfo)
	b.API.CredentialsValidator.RequiresKey = true
	b.API.CredentialsValidator.RequiresSecret = true

	fmt1 := currency.PairStore{
		RequestFormat: &currency.PairFormat{Uppercase: true},
		ConfigFormat:  &currency.PairFormat{Uppercase: true},
	}

	err := b.StoreAssetPairFormat(asset.Spot, fmt1)
	if err != nil {
		log.Errorln(log.ExchangeSys, err)
	}

	b.Features = exchange.Features{
		Supports: exchange.FeaturesSupported{
			REST:      true,
			Websocket: true,
			RESTCapabilities: protocol.Features{
				TickerBatching:      true,
				TickerFetching:      true,
				OrderbookFetching:   true,
				AutoPairUpdates:     true,
				AccountInfo:         true,
				CryptoDeposit:       true,
				CryptoWithdrawal:    true,
				FiatWithdraw:        true,
				GetOrder:            true,
				GetOrders:           true,
				CancelOrders:        true,
				CancelOrder:         true,
				SubmitOrder:         true,
				SubmitOrders:        true,
				DepositHistory:      true,
				WithdrawalHistory:   true,
				TradeFetching:       true,
				UserTradeHistory:    true,
				TradeFee:            true,
				FiatDepositFee:      true,
				FiatWithdrawalFee:   true,
				CryptoDepositFee:    true,
				CryptoWithdrawalFee: true,
			},
			WebsocketCapabilities: protocol.Features{
				AccountBalance:         true,
				CancelOrders:           true,
				CancelOrder:            true,
				SubmitOrder:            true,
				ModifyOrder:            true,
				TickerFetching:         true,
				KlineFetching:          true,
				TradeFetching:          true,
				OrderbookFetching:      true,
				AccountInfo:            true,
				Subscribe:              true,
				AuthenticatedEndpoints: true,
				MessageCorrelation:     true,
				DeadMansSwitch:         true,
				GetOrders:              true,
				GetOrder:               true,
			},
			WithdrawPermissions: exchange.AutoWithdrawCryptoWithAPIPermission |
				exchange.AutoWithdrawFiatWithAPIPermission,
			Kline: kline.ExchangeCapabilitiesSupported{
				DateRanges: true,
				Intervals:  true,
			},
		},
		Enabled: exchange.FeaturesEnabled{
			AutoPairUpdates: true,
			Kline: kline.ExchangeCapabilitiesEnabled{
				Intervals: map[string]bool{
					kline.OneMin.Word():     true,
					kline.ThreeMin.Word():   true,
					kline.FiveMin.Word():    true,
					kline.FifteenMin.Word(): true,
					kline.ThirtyMin.Word():  true,
					kline.OneHour.Word():    true,
					kline.TwoHour.Word():    true,
					kline.FourHour.Word():   true,
					kline.SixHour.Word():    true,
					kline.TwelveHour.Word(): true,
					kline.OneDay.Word():     true,
					kline.OneWeek.Word():    true,
					kline.TwoWeek.Word():    true,
				},
				ResultLimit: 10000,
			},
		},
	}

	b.Requester = request.New(b.Name,
		common.NewHTTPClientWithTimeout(exchange.DefaultHTTPTimeout),
		request.WithLimiter(SetRateLimit()))
	b.API.Endpoints = b.NewEndpoints()
	err = b.API.Endpoints.SetDefaultEndpoints(map[exchange.URL]string{
		exchange.RestSpot:      whitebitAPIURLBase,
		exchange.WebsocketSpot: publicWhitebitWebsocketEndpoint,
	})
	if err != nil {
		log.Errorln(log.ExchangeSys, err)
	}
	b.Websocket = stream.New()
	b.WebsocketResponseMaxLimit = exchange.DefaultWebsocketResponseMaxLimit
	b.WebsocketResponseCheckTimeout = exchange.DefaultWebsocketResponseCheckTimeout
	b.WebsocketOrderbookBufferLimit = exchange.DefaultWebsocketOrderbookBufferLimit
}

// Setup takes in the supplied exchange configuration details and sets params
func (b *Whitebit) Setup(exch *config.ExchangeConfig) error {
	if !exch.Enabled {
		b.SetEnabled(false)
		return nil
	}

	err := b.SetupDefaults(exch)
	if err != nil {
		return err
	}
	wsEndpoint, err := b.API.Endpoints.GetURL(exchange.WebsocketSpot)
	if err != nil {
		return err
	}

	err = b.Websocket.Setup(&stream.WebsocketSetup{
		Enabled:                          exch.Features.Enabled.Websocket,
		Verbose:                          exch.Verbose,
		AuthenticatedWebsocketAPISupport: exch.API.AuthenticatedWebsocketSupport,
		WebsocketTimeout:                 exch.WebsocketTrafficTimeout,
		DefaultURL:                       publicWhitebitWebsocketEndpoint,
		ExchangeName:                     exch.Name,
		RunningURL:                       wsEndpoint,
		Connector:                        b.WsConnect,
		Subscriber:                       b.Subscribe,
		UnSubscriber:                     b.Unsubscribe,
		GenerateSubscriptions:            b.GenerateDefaultSubscriptions,
		Features:                         &b.Features.Supports.WebsocketCapabilities,
		OrderbookBufferLimit:             exch.OrderbookConfig.WebsocketBufferLimit,
		BufferEnabled:                    exch.OrderbookConfig.WebsocketBufferEnabled,
		UpdateEntriesByID:                false,
	})
	if err != nil {
		return err
	}

	err = b.Websocket.SetupNewConnection(stream.ConnectionSetup{
		ResponseCheckTimeout: exch.WebsocketResponseCheckTimeout,
		ResponseMaxLimit:     exch.WebsocketResponseMaxLimit,
		URL:                  publicWhitebitWebsocketEndpoint,
	})
	if err != nil {
		return err
	}

	return b.Websocket.SetupNewConnection(stream.ConnectionSetup{
		ResponseCheckTimeout: exch.WebsocketResponseCheckTimeout,
		ResponseMaxLimit:     exch.WebsocketResponseMaxLimit,
		URL:                  authenticatedWhitebitWebsocketEndpoint,
		Authenticated:        true,
	})
}

// Start starts the Whitebit go routine
func (b *Whitebit) Start(wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		b.Run()
		wg.Done()
	}()
}

// Run implements the Whitebit wrapper
func (b *Whitebit) Run() {
	if b.Verbose {
		log.Debugf(log.ExchangeSys,
			"%s Websocket: %s.",
			b.Name,
			common.IsEnabled(b.Websocket.IsEnabled()))
		b.PrintEnabledPairs()
	}

	if !b.GetEnabledFeatures().AutoPairUpdates {
		return
	}

	err := b.UpdateTradablePairs(false)
	if err != nil {
		log.Errorf(log.ExchangeSys,
			"%s failed to update tradable pairs. Err: %s",
			b.Name,
			err)
	}
}

// FetchTradablePairs returns a list of the exchanges tradable pairs
func (b *Whitebit) FetchTradablePairs(a asset.Item) ([]string, error) {
	items, err := b.GetTickerBatch()
	if err != nil {
		return nil, err
	}

	var symbols []string
	switch a {
	case asset.Spot:
		for k := range items {
			symbols = append(symbols, k)
		}
	default:
		return nil, errors.New("asset type not supported by this endpoint")
	}
//fmt.Println("FetchTradablePairs", a, symbols)
	return symbols, nil
}

// UpdateTradablePairs updates the exchanges available pairs and stores
// them in the exchanges config
func (b *Whitebit) UpdateTradablePairs(forceUpdate bool) error {
	assets := b.CurrencyPairs.GetAssetTypes()
	for i := range assets {
		pairs, err := b.FetchTradablePairs(assets[i])
		if err != nil {
			return err
		}

		p, err := currency.NewPairsFromStrings(pairs)
		if err != nil {
			return err
		}

		err = b.UpdatePairs(p, assets[i], false, forceUpdate)
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateTicker updates and returns the ticker for a currency pair
func (b *Whitebit) UpdateTicker(p currency.Pair, assetType asset.Item) (*ticker.Price, error) {
	enabledPairs, err := b.GetEnabledPairs(assetType)
	if err != nil {
		return nil, err
	}

	tickerNew, err := b.GetTickerBatch()
	if err != nil {
		return nil, err
	}
	//fmt.Println("UpdateTicker, enabledPairs", enabledPairs, tickerNew)
	for k, v := range tickerNew {
		pair, err := currency.NewPairFromString(k)
		if err != nil {
			return nil, err
		}

		if !enabledPairs.Contains(pair, true) {
			continue
		}

		err = ticker.ProcessTicker(&ticker.Price{
			Last:         v.Last,
			High:         v.High,
			Low:          v.Low,
			Bid:          v.Bid,
			Ask:          v.Ask,
			Volume:       v.Volume,
			Pair:         pair,
			AssetType:    assetType,
			ExchangeName: b.Name})
		if err != nil {
			return nil, err
		}
	}
	return ticker.GetTicker(b.Name, p, assetType)
}

// FetchTicker returns the ticker for a currency pair
func (b *Whitebit) FetchTicker(p currency.Pair, assetType asset.Item) (*ticker.Price, error) {
	fPair, err := b.FormatExchangeCurrency(p, assetType)
	if err != nil {
		return nil, err
	}

	b.appendOptionalDelimiter(&fPair)
	tick, err := ticker.GetTicker(b.Name, fPair, asset.Spot)
	if err != nil {
		return b.UpdateTicker(fPair, assetType)
	}
	return tick, nil
}

// FetchOrderbook returns the orderbook for a currency pair
func (b *Whitebit) FetchOrderbook(p currency.Pair, assetType asset.Item) (*orderbook.Base, error) {
	fPair, err := b.FormatExchangeCurrency(p, assetType)
	if err != nil {
		return nil, err
	}

	b.appendOptionalDelimiter(&fPair)
	ob, err := orderbook.Get(b.Name, fPair, assetType)
	if err != nil {
		return b.UpdateOrderbook(fPair, assetType)
	}
	return ob, nil
}

// UpdateOrderbook updates and returns the orderbook for a currency pair
func (b *Whitebit) UpdateOrderbook(p currency.Pair, assetType asset.Item) (*orderbook.Base, error) {
	o := &orderbook.Base{
		ExchangeName:       b.Name,
		Pair:               p,
		AssetType:          assetType,
		NotAggregated:      true,
		VerificationBypass: b.OrderbookVerificationBypass,
	}

	fPair, err := b.FormatExchangeCurrency(p, assetType)
	if err != nil {
		return o, err
	}

	//fmt.Println("XXX", p, assetType, fPair)

	if assetType != asset.Spot {
		return o, fmt.Errorf("assetType not supported: %v", assetType)
	}
	b.appendOptionalDelimiter(&fPair)

	var orderbookNew Orderbook
	orderbookNew, err = b.GetOrderbook(fPair.String(), "100", 2)
	if err != nil {
		return nil, err
	}

	for x := range orderbookNew.Asks {
		o.Asks = append(o.Asks, orderbook.Item{
			ID:     orderbookNew.Asks[x].OrderID,
			Price:  orderbookNew.Asks[x].Price,
			Amount: orderbookNew.Asks[x].Amount,
		})
	}
	for x := range orderbookNew.Bids {
		o.Bids = append(o.Bids, orderbook.Item{
			ID:     orderbookNew.Bids[x].OrderID,
			Price:  orderbookNew.Bids[x].Price,
			Amount: orderbookNew.Bids[x].Amount,
		})
	}

	err = o.Process()
	if err != nil {
		return nil, err
	}
	return orderbook.Get(b.Name, fPair, assetType)
}

// UpdateAccountInfo retrieves balances for all enabled currencies on the
// Whitebit exchange
func (b *Whitebit) UpdateAccountInfo(assetType asset.Item) (account.Holdings, error) {
	var response account.Holdings
	response.Exchange = b.Name

	accountBalance, err := b.GetAccountBalance()
	if err != nil {
		return response, err
	}

	var bl []account.Balance

	for asset, v := range accountBalance {
			bl = append(bl,
				account.Balance{
					CurrencyName: currency.NewCode(asset),
					TotalValue:   v.Available,
					Hold:         v.Freeze,
				})
	}

	response.Accounts = append(response.Accounts, account.SubAccount{
		Currencies: bl,
	})
	err = account.Process(&response)
	if err != nil {
		return account.Holdings{}, err
	}

	response.Exchange = b.Name

	return response, nil
}

// FetchAccountInfo retrieves balances for all enabled currencies
func (b *Whitebit) FetchAccountInfo(assetType asset.Item) (account.Holdings, error) {
	acc, err := account.GetHoldings(b.Name, assetType)
	if err != nil {
		return b.UpdateAccountInfo(assetType)
	}
	return acc, nil
}

// GetFundingHistory returns funding history, deposits and
// withdrawals
func (b *Whitebit) GetFundingHistory() ([]exchange.FundHistory, error) {
	return nil, common.ErrFunctionNotSupported
}

// GetWithdrawalsHistory returns previous withdrawals data
func (b *Whitebit) GetWithdrawalsHistory(c currency.Code) (resp []exchange.WithdrawalHistory, err error) {
	return nil, common.ErrNotYetImplemented
}

// GetRecentTrades returns the most recent trades for a currency and asset
func (b *Whitebit) GetRecentTrades(p currency.Pair, assetType asset.Item) ([]trade.Data, error) {
	return b.GetHistoricTrades(p, assetType, time.Now().Add(-time.Hour), time.Now())
}

// GetHistoricTrades returns historic trade data within the timeframe provided
func (b *Whitebit) GetHistoricTrades(p currency.Pair, assetType asset.Item, timestampStart, timestampEnd time.Time) ([]trade.Data, error) {
	if timestampStart.Equal(timestampEnd) || timestampEnd.After(time.Now()) || timestampEnd.Before(timestampStart) {
		return nil, fmt.Errorf("invalid time range supplied. Start: %v End %v", timestampStart, timestampEnd)
	}
	var err error
	p, err = b.FormatExchangeCurrency(p, assetType)
	if err != nil {
		return nil, err
	}
	var currString string
	currString, err = b.fixCasing(p, assetType)
	if err != nil {
		return nil, err
	}
	var resp []trade.Data
	ts := timestampEnd
	limit := 10000
allTrades:
	for {
		var tradeData []Trade
		tradeData, err = b.GetTrades(currString, int64(limit), 0, ts.Unix()*1000, false)
		if err != nil {
			return nil, err
		}
		for i := range tradeData {
			tradeTS := time.Unix(0, tradeData[i].Timestamp*int64(time.Millisecond))
			if tradeTS.Before(timestampStart) && !timestampStart.IsZero() {
				break allTrades
			}
			tID := strconv.FormatInt(tradeData[i].TID, 10)
			resp = append(resp, trade.Data{
				TID:          tID,
				Exchange:     b.Name,
				CurrencyPair: p,
				AssetType:    assetType,
				Price:        tradeData[i].Price,
				Amount:       tradeData[i].Amount,
				Timestamp:    time.Unix(0, tradeData[i].Timestamp*int64(time.Millisecond)),
			})
			if i == len(tradeData)-1 {
				if ts.Equal(tradeTS) {
					// reached end of trades to crawl
					break allTrades
				}
				ts = tradeTS
			}
		}
		if len(tradeData) != limit {
			break allTrades
		}
	}

	err = b.AddTradesToBuffer(resp...)
	if err != nil {
		return nil, err
	}

	sort.Sort(trade.ByDate(resp))
	return trade.FilterTradesByTime(resp, timestampStart, timestampEnd), nil
}

// SubmitOrder submits a new order
func (b *Whitebit) SubmitOrder(o *order.Submit) (order.SubmitResponse, error) {
	var submitOrderResponse order.SubmitResponse
	err := o.Validate()
	if err != nil {
		return submitOrderResponse, err
	}

	fpair, err := b.FormatExchangeCurrency(o.Pair, o.AssetType)
	if err != nil {
		return submitOrderResponse, err
	}

	if b.Websocket.CanUseAuthenticatedWebsocketForWrapper() {
		fmt.Println("SUBMIT WS")
		submitOrderResponse.OrderID, err = b.WsNewOrder(&WsNewOrderRequest{
			CustomID: b.Websocket.AuthConn.GenerateMessageID(false),
			Type:     o.Type.String(),
			Symbol:   fpair.String(),
			Amount:   o.Amount,
			Price:    o.Price,
		})
		if err != nil {
			return submitOrderResponse, err
		}
	} else {
		fmt.Println("SUBMIT")
		var response Order
		b.appendOptionalDelimiter(&fpair)
		orderType := o.Type.Lower()

		response, err = b.NewOrder(fpair.String(),
			orderType,
			o.Amount,
			o.Price,
			o.Side.String(),
			false)
		if err != nil {
			fmt.Println("ERRRRR:", err)
			return submitOrderResponse, err
		}
		if response.ID > 0 {
			submitOrderResponse.OrderID = strconv.FormatInt(response.ID, 10)
		}
		if response.Left == 0 {
			submitOrderResponse.FullyMatched = true
		}

		submitOrderResponse.IsOrderPlaced = true
	}
	return submitOrderResponse, err
}

// ModifyOrder will allow of changing orderbook placement and limit to
// market conversion
func (b *Whitebit) ModifyOrder(action *order.Modify) (string, error) {
	if err := action.Validate(); err != nil {
		return "", err
	}

	orderIDInt, err := strconv.ParseInt(action.ID, 10, 64)
	if err != nil {
		return action.ID, err
	}
	if b.Websocket.CanUseAuthenticatedWebsocketForWrapper() {
		if action.Side == order.Sell && action.Amount > 0 {
			action.Amount = -1 * action.Amount
		}
		err = b.WsModifyOrder(&WsUpdateOrderRequest{
			OrderID: orderIDInt,
			Price:   action.Price,
			Amount:  action.Amount,
		})
		return action.ID, err
	}
	return "", common.ErrNotYetImplemented
}

// CancelOrder cancels an order by its corresponding ID number
func (b *Whitebit) CancelOrder(o *order.Cancel) error {
	if err := o.Validate(o.StandardCancel()); err != nil {
		return err
	}

	orderIDInt, err := strconv.ParseInt(o.ID, 10, 64)
	if err != nil {
		return err
	}
	if b.Websocket.CanUseAuthenticatedWebsocketForWrapper() {
		err = b.WsCancelOrder(orderIDInt)
	} else {
		var fPair currency.Pair
		fPair, err = b.FormatExchangeCurrency(o.Pair, o.AssetType)
		if err != nil {
			return err
		}

		b.appendOptionalDelimiter(&fPair)
		var resp Order
		resp, err = b.CancelExistingOrder(fPair.String(), orderIDInt)
		if err == nil && resp.Message != "" {
			err = fmt.Errorf(resp.Message)
		}
	}
	return err
}


// CancelBatchOrders cancels an orders by their corresponding ID numbers
func (b *Whitebit) CancelBatchOrders(o []order.Cancel) (order.CancelBatchResponse, error) {
	return order.CancelBatchResponse{}, common.ErrNotYetImplemented
}

// CancelAllOrders cancels all orders associated with a currency pair
func (b *Whitebit) CancelAllOrders(_ *order.Cancel) (order.CancelAllResponse, error) {
	var err error
	if b.Websocket.CanUseAuthenticatedWebsocketForWrapper() {
		err = b.WsCancelAllOrders()
	} else {
		_, err = b.CancelAllExistingOrders()
	}
	return order.CancelAllResponse{}, err
}

// GetOrderInfo returns order information based on order ID
func (b *Whitebit) GetOrderInfo(orderID string, pair currency.Pair, assetType asset.Item) (order.Detail, error) {
	var orderDetail order.Detail
	//return orderDetail, common.ErrNotYetImplemented
	//fPair, err := b.FormatExchangeCurrency(pair, assetType)
	//if err != nil {
	//	return orderDetail, err
	//}
	//
	//b.appendOptionalDelimiter(&fPair)

	ID, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return orderDetail, err
	}
	//fmt.Println("GetOrderStatus11")
	o, err := b.GetOrderStatus(ID)
	//fmt.Printf("GetOrderStatus 2 :::: %+v\n", o)
	if err == nil {
		if o.Message != "" {
			err = fmt.Errorf(o.Message)
		} else {
			msgs := CheckErrMsgs(o.Errors)
			if msgs != "" {
				err = fmt.Errorf(msgs)
			}
		}
	}

	if err != nil {
		return orderDetail, err
	}

	var trades []order.TradeHistory

	orderDetail.AssetType = asset.Spot
	for i := range o.Records {
		//orderDetail.Pair = o.Result.Records[i].
		orderDetail.Amount += o.Records[i].Amount
		orderDetail.Cost += o.Records[i].Deal

		var IsMaker bool
		if o.Records[i].Role == 1 {
			IsMaker = true
		}

		trades = append(trades, order.TradeHistory{
			Price: o.Records[i].Price,
			Amount: o.Records[i].Amount,
			Fee: o.Records[i].Fee,
			Exchange: b.Name,
			TID: fmt.Sprint(o.Records[i].ID),
			// Type: o.Result.Records[i].
			// Side: o.Result.Records[i].
			Timestamp: time.Unix(int64(o.Records[i].Time), 0),
			IsMaker: IsMaker,
			Total: o.Records[i].Deal, // ?
		})
	}

	orderDetail.Pair = pair
	orderDetail.Trades = trades
	orderDetail.Exchange = b.Name
	fmt.Printf("GetOrderStatus:::: %+v\n", orderDetail)
	return orderDetail, nil
}

// GetDepositAddress returns a deposit address for a specified currency
func (b *Whitebit) GetDepositAddress(c currency.Code, accountID string) (string, error) {
	return "", common.ErrNotYetImplemented
	//if accountID == "" {
	//	accountID = "deposit"
	//}

	//method, err := b.ConvertSymbolToDepositMethod(c)
	//if err != nil {
	//	return "", err
	//}

	//fmt.Println("GetDepositAddress")
	//time.Sleep(time.Hour)
	//
	//resp, err := b.NewDeposit(method, accountID, 0)
	//return resp.Address, err
}

// WithdrawCryptocurrencyFunds returns a withdrawal ID when a withdrawal is submitted
func (b *Whitebit) WithdrawCryptocurrencyFunds(withdrawRequest *withdraw.Request) (*withdraw.ExchangeResponse, error) {
	if err := withdrawRequest.Validate(); err != nil {
		return nil, err
	}

	// Whitebit has support for three types, exchange, margin and deposit
	// As this is for trading, I've made the wrapper default 'exchange'
	// TODO: Discover an automated way to make the decision for wallet type to withdraw from
	walletType := "exchange"
	resp, err := b.WithdrawCryptocurrency(walletType,
		withdrawRequest.Crypto.Address,
		withdrawRequest.Description,
		withdrawRequest.Amount,
		withdrawRequest.Currency)
	if err != nil {
		return nil, err
	}

	return &withdraw.ExchangeResponse{
		ID:     strconv.FormatInt(resp.WithdrawalID, 10),
		Status: resp.Status,
	}, err
}

// WithdrawFiatFunds returns a withdrawal ID when a withdrawal is submitted
// Returns comma delimited withdrawal IDs
func (b *Whitebit) WithdrawFiatFunds(withdrawRequest *withdraw.Request) (*withdraw.ExchangeResponse, error) {
	if err := withdrawRequest.Validate(); err != nil {
		return nil, err
	}

	withdrawalType := "wire"
	// Whitebit has support for three types, exchange, margin and deposit
	// As this is for trading, I've made the wrapper default 'exchange'
	// TODO: Discover an automated way to make the decision for wallet type to withdraw from
	walletType := "exchange"
	resp, err := b.WithdrawFIAT(withdrawalType, walletType, withdrawRequest)
	if err != nil {
		return nil, err
	}

	return &withdraw.ExchangeResponse{
		ID:     strconv.FormatInt(resp.WithdrawalID, 10),
		Status: resp.Status,
	}, err
}

// WithdrawFiatFundsToInternationalBank returns a withdrawal ID when a withdrawal is submitted
// Returns comma delimited withdrawal IDs
func (b *Whitebit) WithdrawFiatFundsToInternationalBank(withdrawRequest *withdraw.Request) (*withdraw.ExchangeResponse, error) {
	if err := withdrawRequest.Validate(); err != nil {
		return nil, err
	}

	v, err := b.WithdrawFiatFunds(withdrawRequest)
	if err != nil {
		return nil, err
	}
	return &withdraw.ExchangeResponse{
		ID:     v.ID,
		Status: v.Status,
	}, nil
}

// GetFeeByType returns an estimate of fee based on type of transaction
func (b *Whitebit) GetFeeByType(feeBuilder *exchange.FeeBuilder) (float64, error) {
	if !b.AllowAuthenticatedRequest() && // Todo check connection status
		feeBuilder.FeeType == exchange.CryptocurrencyTradeFee {
		feeBuilder.FeeType = exchange.OfflineTradeFee
	}
	return b.GetFee(feeBuilder)
}

// GetActiveOrders retrieves any orders that are active/open
func (b *Whitebit) GetActiveOrders(req *order.GetOrdersRequest) ([]order.Detail, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	if len(req.Pairs) == 0 {
		return nil, fmt.Errorf("pair not passed")
	}

	var orders []order.Detail
	b.appendOptionalDelimiter(&req.Pairs[0])
	resp, err := b.GetOpenOrders(req.Pairs[0].String())
	if err != nil {
		return nil, err
	}

	fmt.Printf("GetOpenOrders: %+v\n", resp)

	for i := range resp {
		orderSide := order.Side(strings.ToUpper(resp[i].Side))

		pair, err := currency.NewPairFromString(resp[i].Market)
		if err != nil {
			return nil, err
		}

		orderDetail := order.Detail{
			Amount:          resp[i].Amount,
			Date:            time.Unix(int64(resp[i].Timestamp), 0),
			Exchange:        b.Name,
			ID:              strconv.FormatInt(resp[i].ID, 10),
			Side:            orderSide,
			Price:           resp[i].Price,
			RemainingAmount: resp[i].Left,
			Pair:            pair,
			ExecutedAmount:  resp[i].DealMoney, // ??? need check
		}

		orderDetail.Status = order.Active

		// API docs discrepancy. Example contains prefixed "exchange "
		// Return type suggests “market” / “limit” / “stop” / “trailing-stop”
		orderType := strings.Replace(resp[i].Type, "exchange ", "", 1)
		if orderType == "trailing-stop" {
			orderDetail.Type = order.TrailingStop
		} else {
			orderDetail.Type = order.Type(strings.ToUpper(orderType))
		}

		orders = append(orders, orderDetail)
	}

	order.FilterOrdersBySide(&orders, req.Side)
	order.FilterOrdersByType(&orders, req.Type)
	//order.FilterOrdersByTickRange(&orders, req.StartTicks, req.EndTicks)
	order.FilterOrdersByCurrencies(&orders, req.Pairs)
	return orders, nil
}

// GetOrderHistory retrieves account order information
// Can Limit response to specific order status
func (b *Whitebit) GetOrderHistory(req *order.GetOrdersRequest) ([]order.Detail, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	var orders []order.Detail
	resp, err := b.GetInactiveOrders()
	if err != nil {
		return nil, err
	}

	for i := range resp {
		orderSide := order.Side(strings.ToUpper(resp[i].Side))

		orderDate := time.Unix(int64(resp[i].Timestamp), 0)

		pair, err := currency.NewPairFromString(resp[i].Market)
		if err != nil {
			return nil, err
		}

		orderDetail := order.Detail{
			Amount:          resp[i].Amount,
			Date:            orderDate,
			Exchange:        b.Name,
			ID:              strconv.FormatInt(resp[i].ID, 10),
			Side:            orderSide,
			Price:           resp[i].Price,
			RemainingAmount: resp[i].Left,
			Pair:            pair,
		}

		//switch {
		//case resp[i].IsLive:
		//	orderDetail.Status = order.Active
		//case resp[i].IsCancelled:
		//	orderDetail.Status = order.Cancelled
		//default:
		//	orderDetail.Status = order.UnknownStatus
		//}

		switch resp[i].Type {
		case "market":
			orderDetail.Type = order.Market
		case "limit":
			orderDetail.Type = order.Limit
		case "stop-limit":
			orderDetail.Type = order.StopLimit
		case "stop-market":
			orderDetail.Type = order.StopMarket
		}

		orders = append(orders, orderDetail)
	}

	order.FilterOrdersBySide(&orders, req.Side)
	order.FilterOrdersByType(&orders, req.Type)
	//order.FilterOrdersByTickRange(&orders, req.StartTicks, req.EndTicks)
	for i := range req.Pairs {
		b.appendOptionalDelimiter(&req.Pairs[i])
	}
	order.FilterOrdersByCurrencies(&orders, req.Pairs)
	return orders, nil
}

// AuthenticateWebsocket sends an authentication message to the websocket
func (b *Whitebit) AuthenticateWebsocket() error {
	resp, err := b.GetWebsocketToken()
	if resp != "" {
		authToken = resp
	}
	return err
}

// appendOptionalDelimiter ensures that a delimiter is present for long character currencies
func (b *Whitebit) appendOptionalDelimiter(p *currency.Pair) {
	//if len(p.Quote.String()) > 3 || len(p.Base.String()) > 3 {
		p.Delimiter = "_"
	//}
}

// ValidateCredentials validates current credentials used for wrapper
// functionality
func (b *Whitebit) ValidateCredentials(assetType asset.Item) error {
	_, err := b.UpdateAccountInfo(assetType)
	return b.CheckTransientError(err)
}

// FormatExchangeKlineInterval returns Interval to exchange formatted string
func (b *Whitebit) FormatExchangeKlineInterval(in kline.Interval) string {
	switch in {
	case kline.OneDay:
		return "1D"
	case kline.OneWeek:
		return "7D"
	case kline.OneWeek * 2:
		return "14D"
	default:
		return in.Short()
	}
}

// GetHistoricCandles returns candles between a time period for a set time interval
func (b *Whitebit) GetHistoricCandles(pair currency.Pair, a asset.Item, start, end time.Time, interval kline.Interval) (kline.Item, error) {
	if err := b.ValidateKline(pair, a, interval); err != nil {
		return kline.Item{}, err
	}

	if kline.TotalCandlesPerInterval(start, end, interval) > b.Features.Enabled.Kline.ResultLimit {
		return kline.Item{}, errors.New(kline.ErrRequestExceedsExchangeLimits)
	}

	cf, err := b.fixCasing(pair, a)
	if err != nil {
		return kline.Item{}, err
	}

	candles, err := b.GetCandles(cf, b.FormatExchangeKlineInterval(interval),
		start.Unix()*1000, end.Unix()*1000,
		b.Features.Enabled.Kline.ResultLimit, true)
	if err != nil {
		return kline.Item{}, err
	}
	ret := kline.Item{
		Exchange: b.Name,
		Pair:     pair,
		Asset:    a,
		Interval: interval,
	}

	for x := range candles {
		ret.Candles = append(ret.Candles, kline.Candle{
			Time:   candles[x].Timestamp,
			Open:   candles[x].Open,
			High:   candles[x].High,
			Low:    candles[x].Low,
			Close:  candles[x].Close,
			Volume: candles[x].Volume,
		})
	}

	ret.SortCandlesByTimestamp(false)
	return ret, nil
}

// GetHistoricCandlesExtended returns candles between a time period for a set time interval
func (b *Whitebit) GetHistoricCandlesExtended(pair currency.Pair, a asset.Item, start, end time.Time, interval kline.Interval) (kline.Item, error) {
	if err := b.ValidateKline(pair, a, interval); err != nil {
		return kline.Item{}, err
	}

	ret := kline.Item{
		Exchange: b.Name,
		Pair:     pair,
		Asset:    a,
		Interval: interval,
	}

	dates := kline.CalcDateRanges(start, end, interval, b.Features.Enabled.Kline.ResultLimit)
	cf, err := b.fixCasing(pair, a)
	if err != nil {
		return kline.Item{}, err
	}

	for x := range dates {
		candles, err := b.GetCandles(cf, b.FormatExchangeKlineInterval(interval),
			dates[x].Start.Unix()*1000, dates[x].End.Unix()*1000,
			b.Features.Enabled.Kline.ResultLimit, true)
		if err != nil {
			return kline.Item{}, err
		}

		for i := range candles {
			ret.Candles = append(ret.Candles, kline.Candle{
				Time:   candles[i].Timestamp,
				Open:   candles[i].Open,
				High:   candles[i].High,
				Low:    candles[i].Low,
				Close:  candles[i].Close,
				Volume: candles[i].Volume,
			})
		}
	}

	ret.SortCandlesByTimestamp(false)
	return ret, nil
}

func (b *Whitebit) fixCasing(in currency.Pair, a asset.Item) (string, error) {
	var checkString [2]byte
	if a == asset.Spot {
		checkString[0] = 't'
		checkString[1] = 'T'
	}

	fmt, err := b.FormatExchangeCurrency(in, a)
	if err != nil {
		return "", err
	}

	y := in.Base.String()
	if (y[0] != checkString[0] && y[0] != checkString[1]) ||
		(y[0] == checkString[1] && y[1] == checkString[1]) || in.Base == currency.TNB {
		if fmt.Quote.IsEmpty() {
			return string(checkString[0]) + fmt.Base.Upper().String(), nil
		}
		return string(checkString[0]) + fmt.Upper().String(), nil
	}

	runes := []rune(fmt.Upper().String())
	if fmt.Quote.IsEmpty() {
		runes = []rune(fmt.Base.Upper().String())
	}
	runes[0] = unicode.ToLower(runes[0])
	return string(runes), nil
}


func CheckErrMsgs (e Errors) (errMsg string) {
	switch {
	case len(e.Limit) > 0:
		for i := range e.Limit {
			if len(e.Limit) == i + 1 {
				errMsg += e.Limit[i]
			} else {
				errMsg += e.Limit[i] + ", "
			}
		}
	case len(e.Market) > 0:
		for i := range e.Market {
			if len(e.Market) == i + 1 {
				errMsg += e.Market[i]
			} else {
				errMsg += e.Market[i] + ", "
			}
		}
	case len(e.OrderId) > 0:
		for i := range e.OrderId {
			if len(e.OrderId) == i + 1 {
				errMsg += e.OrderId[i]
			} else {
				errMsg += e.OrderId[i] + ", "
			}
		}
	case len(e.Offset) > 0:
		for i := range e.Offset {
			if len(e.Offset) == i + 1 {
				errMsg += e.Offset[i]
			} else {
				errMsg += e.Offset[i] + ", "
			}
		}
	}

	return
}