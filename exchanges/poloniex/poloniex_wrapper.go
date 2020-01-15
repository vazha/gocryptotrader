package poloniex

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vazha/gocryptotrader/common"
	"github.com/vazha/gocryptotrader/config"
	"github.com/vazha/gocryptotrader/currency"
	exchange "github.com/vazha/gocryptotrader/exchanges"
	"github.com/vazha/gocryptotrader/exchanges/asset"
	"github.com/vazha/gocryptotrader/exchanges/order"
	"github.com/vazha/gocryptotrader/exchanges/orderbook"
	"github.com/vazha/gocryptotrader/exchanges/protocol"
	"github.com/vazha/gocryptotrader/exchanges/request"
	"github.com/vazha/gocryptotrader/exchanges/ticker"
	"github.com/vazha/gocryptotrader/exchanges/websocket/wshandler"
	"github.com/vazha/gocryptotrader/exchanges/withdraw"
	log "github.com/vazha/gocryptotrader/logger"
)

// GetDefaultConfig returns a default exchange config
func (p *Poloniex) GetDefaultConfig() (*config.ExchangeConfig, error) {
	p.SetDefaults()
	exchCfg := new(config.ExchangeConfig)
	exchCfg.Name = p.Name
	exchCfg.HTTPTimeout = exchange.DefaultHTTPTimeout
	exchCfg.BaseCurrencies = p.BaseCurrencies

	err := p.SetupDefaults(exchCfg)
	if err != nil {
		return nil, err
	}

	if p.Features.Supports.RESTCapabilities.AutoPairUpdates {
		err = p.UpdateTradablePairs(true)
		if err != nil {
			return nil, err
		}
	}

	return exchCfg, nil
}

// SetDefaults sets default settings for poloniex
func (p *Poloniex) SetDefaults() {
	p.Name = "Poloniex"
	p.Enabled = true
	p.Verbose = true
	p.API.CredentialsValidator.RequiresKey = true
	p.API.CredentialsValidator.RequiresSecret = true

	p.CurrencyPairs = currency.PairsManager{
		AssetTypes: asset.Items{
			asset.Spot,
		},
		UseGlobalFormat: true,
		RequestFormat: &currency.PairFormat{
			Delimiter: delimiterUnderscore,
			Uppercase: true,
		},
		ConfigFormat: &currency.PairFormat{
			Delimiter: delimiterUnderscore,
			Uppercase: true,
		},
	}

	p.Features = exchange.Features{
		Supports: exchange.FeaturesSupported{
			REST:      true,
			Websocket: true,
			RESTCapabilities: protocol.Features{
				TickerBatching:      true,
				TickerFetching:      true,
				KlineFetching:       true,
				TradeFetching:       true,
				OrderbookFetching:   true,
				AutoPairUpdates:     true,
				AccountInfo:         true,
				GetOrder:            true,
				GetOrders:           true,
				CancelOrder:         true,
				CancelOrders:        true,
				SubmitOrder:         true,
				DepositHistory:      true,
				WithdrawalHistory:   true,
				UserTradeHistory:    true,
				CryptoDeposit:       true,
				CryptoWithdrawal:    true,
				TradeFee:            true,
				CryptoWithdrawalFee: true,
			},
			WebsocketCapabilities: protocol.Features{
				TickerFetching:         true,
				TradeFetching:          true,
				OrderbookFetching:      true,
				Subscribe:              true,
				Unsubscribe:            true,
				AuthenticatedEndpoints: true,
			},
			WithdrawPermissions: exchange.AutoWithdrawCryptoWithAPIPermission |
				exchange.NoFiatWithdrawals,
		},
		Enabled: exchange.FeaturesEnabled{
			AutoPairUpdates: true,
		},
	}

	p.Requester = request.New(p.Name,
		request.NewRateLimit(time.Second, poloniexAuthRate),
		request.NewRateLimit(time.Second, poloniexUnauthRate),
		common.NewHTTPClientWithTimeout(exchange.DefaultHTTPTimeout))

	p.API.Endpoints.URLDefault = poloniexAPIURL
	p.API.Endpoints.URL = p.API.Endpoints.URLDefault
	p.API.Endpoints.WebsocketURL = poloniexWebsocketAddress
	p.Websocket = wshandler.New()
	p.WebsocketResponseMaxLimit = exchange.DefaultWebsocketResponseMaxLimit
	p.WebsocketResponseCheckTimeout = exchange.DefaultWebsocketResponseCheckTimeout
	p.WebsocketOrderbookBufferLimit = exchange.DefaultWebsocketOrderbookBufferLimit
}

// Setup sets user exchange configuration settings
func (p *Poloniex) Setup(exch *config.ExchangeConfig) error {
	if !exch.Enabled {
		p.SetEnabled(false)
		return nil
	}

	err := p.SetupDefaults(exch)
	if err != nil {
		return err
	}

	err = p.Websocket.Setup(
		&wshandler.WebsocketSetup{
			Enabled:                          exch.Features.Enabled.Websocket,
			Verbose:                          exch.Verbose,
			AuthenticatedWebsocketAPISupport: exch.API.AuthenticatedWebsocketSupport,
			WebsocketTimeout:                 exch.WebsocketTrafficTimeout,
			DefaultURL:                       poloniexWebsocketAddress,
			ExchangeName:                     exch.Name,
			RunningURL:                       exch.API.Endpoints.WebsocketURL,
			Connector:                        p.WsConnect,
			Subscriber:                       p.Subscribe,
			UnSubscriber:                     p.Unsubscribe,
			Features:                         &p.Features.Supports.WebsocketCapabilities,
		})
	if err != nil {
		return err
	}

	p.WebsocketConn = &wshandler.WebsocketConnection{
		ExchangeName:         p.Name,
		URL:                  p.Websocket.GetWebsocketURL(),
		ProxyURL:             p.Websocket.GetProxyAddress(),
		Verbose:              p.Verbose,
		ResponseCheckTimeout: exch.WebsocketResponseCheckTimeout,
		ResponseMaxLimit:     exch.WebsocketResponseMaxLimit,
	}

	p.Websocket.Orderbook.Setup(
		exch.WebsocketOrderbookBufferLimit,
		false,
		true,
		true,
		false,
		exch.Name)
	return nil
}

// Start starts the Poloniex go routine
func (p *Poloniex) Start(wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		p.Run()
		wg.Done()
	}()
}

// Run implements the Poloniex wrapper
func (p *Poloniex) Run() {
	if p.Verbose {
		log.Debugf(log.ExchangeSys, "%s Websocket: %s (url: %s).\n", p.Name, common.IsEnabled(p.Websocket.IsEnabled()), poloniexWebsocketAddress)
		p.PrintEnabledPairs()
	}

	forceUpdate := false
	if common.StringDataCompare(p.GetAvailablePairs(asset.Spot).Strings(), "BTC_USDT") {
		log.Warnf(log.ExchangeSys, "%s contains invalid pair, forcing upgrade of available currencies.\n",
			p.Name)
		forceUpdate = true
	}

	if !p.GetEnabledFeatures().AutoPairUpdates && !forceUpdate {
		return
	}

	err := p.UpdateTradablePairs(forceUpdate)
	if err != nil {
		log.Errorf(log.ExchangeSys, "%s failed to update tradable pairs. Err: %s", p.Name, err)
	}
}

// FetchTradablePairs returns a list of the exchanges tradable pairs
func (p *Poloniex) FetchTradablePairs(asset asset.Item) ([]string, error) {
	resp, err := p.GetTicker()
	if err != nil {
		return nil, err
	}

	var currencies []string
	for x := range resp {
		currencies = append(currencies, x)
	}

	return currencies, nil
}

// UpdateTradablePairs updates the exchanges available pairs and stores
// them in the exchanges config
func (p *Poloniex) UpdateTradablePairs(forceUpgrade bool) error {
	pairs, err := p.FetchTradablePairs(asset.Spot)
	if err != nil {
		return err
	}

	return p.UpdatePairs(currency.NewPairsFromStrings(pairs), asset.Spot, false, forceUpgrade)
}

// UpdateTicker updates and returns the ticker for a currency pair
func (p *Poloniex) UpdateTicker(currencyPair currency.Pair, assetType asset.Item) (*ticker.Price, error) {
	tickerPrice := new(ticker.Price)
	tick, err := p.GetTicker()
	if err != nil {
		return tickerPrice, err
	}

	enabledPairs := p.GetEnabledPairs(assetType)
	for i := range enabledPairs {
		var tp ticker.Price
		curr := p.FormatExchangeCurrency(enabledPairs[i], assetType).String()
		if _, ok := tick[curr]; !ok {
			continue
		}
		tp.Pair = enabledPairs[i]
		tp.Ask = tick[curr].LowestAsk
		tp.Bid = tick[curr].HighestBid
		tp.High = tick[curr].High24Hr
		tp.Last = tick[curr].Last
		tp.Low = tick[curr].Low24Hr
		tp.Volume = tick[curr].BaseVolume
		tp.QuoteVolume = tick[curr].QuoteVolume

		err = ticker.ProcessTicker(p.Name, &tp, assetType)
		if err != nil {
			log.Error(log.Ticker, err)
		}
	}
	return ticker.GetTicker(p.Name, currencyPair, assetType)
}

// FetchTicker returns the ticker for a currency pair
func (p *Poloniex) FetchTicker(currencyPair currency.Pair, assetType asset.Item) (*ticker.Price, error) {
	tickerNew, err := ticker.GetTicker(p.Name, currencyPair, assetType)
	if err != nil {
		return p.UpdateTicker(currencyPair, assetType)
	}
	return tickerNew, nil
}

// FetchOrderbook returns orderbook base on the currency pair
func (p *Poloniex) FetchOrderbook(currencyPair currency.Pair, assetType asset.Item) (*orderbook.Base, error) {
	ob, err := orderbook.Get(p.Name, currencyPair, assetType)
	if err != nil {
		return p.UpdateOrderbook(currencyPair, assetType)
	}
	return ob, nil
}

// UpdateOrderbook updates and returns the orderbook for a currency pair
func (p *Poloniex) UpdateOrderbook(currencyPair currency.Pair, assetType asset.Item) (*orderbook.Base, error) {
	orderBook := new(orderbook.Base)
	orderbookNew, err := p.GetOrderbook("", 1000)
	if err != nil {
		return orderBook, err
	}

	enabledPairs := p.GetEnabledPairs(assetType)
	for i := range enabledPairs {
		data, ok := orderbookNew.Data[p.FormatExchangeCurrency(enabledPairs[i], assetType).String()]
		if !ok {
			continue
		}

		var obItems []orderbook.Item
		for y := range data.Bids {
			obItems = append(obItems, orderbook.Item{
				Amount: data.Bids[y].Amount, Price: data.Bids[y].Price})
		}
		orderBook.Bids = obItems

		obItems = []orderbook.Item{}
		for y := range data.Asks {
			obItems = append(obItems, orderbook.Item{
				Amount: data.Asks[y].Amount, Price: data.Asks[y].Price})
		}
		orderBook.Asks = obItems
		orderBook.Pair = enabledPairs[i]
		orderBook.ExchangeName = p.Name
		orderBook.AssetType = assetType

		err = orderBook.Process()
		if err != nil {
			return orderBook, err
		}
	}
	return orderbook.Get(p.Name, currencyPair, assetType)
}

// GetAccountInfo retrieves balances for all enabled currencies for the
// Poloniex exchange
func (p *Poloniex) GetAccountInfo() (exchange.AccountInfo, error) {
	var response exchange.AccountInfo
	response.Exchange = p.Name
	accountBalance, err := p.GetBalances()
	if err != nil {
		return response, err
	}

	var currencies []exchange.AccountCurrencyInfo
	for x, y := range accountBalance.Currency {
		var exchangeCurrency exchange.AccountCurrencyInfo
		exchangeCurrency.CurrencyName = currency.NewCode(x)
		exchangeCurrency.TotalValue = y
		currencies = append(currencies, exchangeCurrency)
	}

	response.Accounts = append(response.Accounts, exchange.Account{
		Currencies: currencies,
	})

	return response, nil
}

// GetFundingHistory returns funding history, deposits and
// withdrawals
func (p *Poloniex) GetFundingHistory() ([]exchange.FundHistory, error) {
	return nil, common.ErrFunctionNotSupported
}

// GetExchangeHistory returns historic trade data since exchange opening.
func (p *Poloniex) GetExchangeHistory(currencyPair currency.Pair, assetType asset.Item) ([]exchange.TradeHistory, error) {
	return nil, common.ErrNotYetImplemented
}

// SubmitOrder submits a new order
func (p *Poloniex) SubmitOrder(s *order.Submit) (order.SubmitResponse, error) {
	var submitOrderResponse order.SubmitResponse
	if err := s.Validate(); err != nil {
		return submitOrderResponse, err
	}

	fillOrKill := s.OrderType == order.Market
	isBuyOrder := s.OrderSide == order.Buy
	response, err := p.PlaceOrder(s.Pair.String(),
		s.Price,
		s.Amount,
		false,
		fillOrKill,
		isBuyOrder)
	if err != nil {
		return submitOrderResponse, err
	}
	if response.OrderNumber > 0 {
		submitOrderResponse.OrderID = strconv.FormatInt(response.OrderNumber, 10)
	}

	submitOrderResponse.IsOrderPlaced = true
	if s.OrderType == order.Market {
		submitOrderResponse.FullyMatched = true
	}
	return submitOrderResponse, nil
}

// ModifyOrder will allow of changing orderbook placement and limit to
// market conversion
func (p *Poloniex) ModifyOrder(action *order.Modify) (string, error) {
	oID, err := strconv.ParseInt(action.OrderID, 10, 64)
	if err != nil {
		return "", err
	}

	resp, err := p.MoveOrder(oID,
		action.Price,
		action.Amount,
		action.PostOnly,
		action.ImmediateOrCancel)
	if err != nil {
		return "", err
	}

	return strconv.FormatInt(resp.OrderNumber, 10), nil
}

// CancelOrder cancels an order by its corresponding ID number
func (p *Poloniex) CancelOrder(order *order.Cancel) error {
	orderIDInt, err := strconv.ParseInt(order.OrderID, 10, 64)
	if err != nil {
		return err
	}

	return p.CancelExistingOrder(orderIDInt)
}

// CancelAllOrders cancels all orders associated with a currency pair
func (p *Poloniex) CancelAllOrders(_ *order.Cancel) (order.CancelAllResponse, error) {
	cancelAllOrdersResponse := order.CancelAllResponse{
		Status: make(map[string]string),
	}
	openOrders, err := p.GetOpenOrdersForAllCurrencies()
	if err != nil {
		return cancelAllOrdersResponse, err
	}

	for key := range openOrders.Data {
		for i := range openOrders.Data[key] {
			err = p.CancelExistingOrder(openOrders.Data[key][i].OrderNumber)
			if err != nil {
				id := strconv.FormatInt(openOrders.Data[key][i].OrderNumber, 10)
				cancelAllOrdersResponse.Status[id] = err.Error()
			}
		}
	}

	return cancelAllOrdersResponse, nil
}

// GetOrderInfo returns information on a current open order
func (p *Poloniex) GetOrderInfo(orderID string) (order.Detail, error) {
	var orderDetail order.Detail
	return orderDetail, common.ErrNotYetImplemented
}

// GetDepositAddress returns a deposit address for a specified currency
func (p *Poloniex) GetDepositAddress(cryptocurrency currency.Code, _ string) (string, error) {
	a, err := p.GetDepositAddresses()
	if err != nil {
		return "", err
	}

	address, ok := a.Addresses[cryptocurrency.Upper().String()]
	if !ok {
		return "", fmt.Errorf("cannot find deposit address for %s",
			cryptocurrency)
	}

	return address, nil
}

// WithdrawCryptocurrencyFunds returns a withdrawal ID when a withdrawal is
// submitted
func (p *Poloniex) WithdrawCryptocurrencyFunds(withdrawRequest *withdraw.CryptoRequest) (string, error) {
	_, err := p.Withdraw(withdrawRequest.Currency.String(), withdrawRequest.Address, withdrawRequest.Amount)
	return "", err
}

// WithdrawFiatFunds returns a withdrawal ID when a
// withdrawal is submitted
func (p *Poloniex) WithdrawFiatFunds(withdrawRequest *withdraw.FiatRequest) (string, error) {
	return "", common.ErrFunctionNotSupported
}

// WithdrawFiatFundsToInternationalBank returns a withdrawal ID when a
// withdrawal is submitted
func (p *Poloniex) WithdrawFiatFundsToInternationalBank(withdrawRequest *withdraw.FiatRequest) (string, error) {
	return "", common.ErrFunctionNotSupported
}

// GetWebsocket returns a pointer to the exchange websocket
func (p *Poloniex) GetWebsocket() (*wshandler.Websocket, error) {
	return p.Websocket, nil
}

// GetFeeByType returns an estimate of fee based on type of transaction
func (p *Poloniex) GetFeeByType(feeBuilder *exchange.FeeBuilder) (float64, error) {
	if (!p.AllowAuthenticatedRequest() || p.SkipAuthCheck) && // Todo check connection status
		feeBuilder.FeeType == exchange.CryptocurrencyTradeFee {
		feeBuilder.FeeType = exchange.OfflineTradeFee
	}
	return p.GetFee(feeBuilder)
}

// GetActiveOrders retrieves any orders that are active/open
func (p *Poloniex) GetActiveOrders(req *order.GetOrdersRequest) ([]order.Detail, error) {
	resp, err := p.GetOpenOrdersForAllCurrencies()
	if err != nil {
		return nil, err
	}

	var orders []order.Detail
	for key := range resp.Data {
		symbol := currency.NewPairDelimiter(key,
			p.GetPairFormat(asset.Spot, false).Delimiter)

		for i := range resp.Data[key] {
			orderSide := order.Side(strings.ToUpper(resp.Data[key][i].Type))
			orderDate, err := time.Parse(poloniexDateLayout, resp.Data[key][i].Date)
			if err != nil {
				log.Errorf(log.ExchangeSys,
					"Exchange %v Func %v Order %v Could not parse date to unix with value of %v",
					p.Name,
					"GetActiveOrders",
					resp.Data[key][i].OrderNumber,
					resp.Data[key][i].Date)
			}

			orders = append(orders, order.Detail{
				ID:           strconv.FormatInt(resp.Data[key][i].OrderNumber, 10),
				OrderSide:    orderSide,
				Amount:       resp.Data[key][i].Amount,
				OrderDate:    orderDate,
				Price:        resp.Data[key][i].Rate,
				CurrencyPair: symbol,
				Exchange:     p.Name,
			})
		}
	}

	order.FilterOrdersByTickRange(&orders, req.StartTicks, req.EndTicks)
	order.FilterOrdersByCurrencies(&orders, req.Currencies)
	order.FilterOrdersBySide(&orders, req.OrderSide)

	return orders, nil
}

// GetOrderHistory retrieves account order information
// Can Limit response to specific order status
func (p *Poloniex) GetOrderHistory(req *order.GetOrdersRequest) ([]order.Detail, error) {
	resp, err := p.GetAuthenticatedTradeHistory(req.StartTicks.Unix(),
		req.EndTicks.Unix(),
		10000)
	if err != nil {
		return nil, err
	}

	var orders []order.Detail
	for key := range resp.Data {
		symbol := currency.NewPairDelimiter(key,
			p.GetPairFormat(asset.Spot, false).Delimiter)

		for i := range resp.Data[key] {
			orderSide := order.Side(strings.ToUpper(resp.Data[key][i].Type))
			orderDate, err := time.Parse(poloniexDateLayout,
				resp.Data[key][i].Date)
			if err != nil {
				log.Errorf(log.ExchangeSys,
					"Exchange %v Func %v Order %v Could not parse date to unix with value of %v",
					p.Name,
					"GetActiveOrders",
					resp.Data[key][i].OrderNumber,
					resp.Data[key][i].Date)
			}

			orders = append(orders, order.Detail{
				ID:           strconv.FormatInt(resp.Data[key][i].GlobalTradeID, 10),
				OrderSide:    orderSide,
				Amount:       resp.Data[key][i].Amount,
				OrderDate:    orderDate,
				Price:        resp.Data[key][i].Rate,
				CurrencyPair: symbol,
				Exchange:     p.Name,
			})
		}
	}

	order.FilterOrdersByCurrencies(&orders, req.Currencies)
	order.FilterOrdersBySide(&orders, req.OrderSide)

	return orders, nil
}

// SubscribeToWebsocketChannels appends to ChannelsToSubscribe
// which lets websocket.manageSubscriptions handle subscribing
func (p *Poloniex) SubscribeToWebsocketChannels(channels []wshandler.WebsocketChannelSubscription) error {
	p.Websocket.SubscribeToChannels(channels)
	return nil
}

// UnsubscribeToWebsocketChannels removes from ChannelsToSubscribe
// which lets websocket.manageSubscriptions handle unsubscribing
func (p *Poloniex) UnsubscribeToWebsocketChannels(channels []wshandler.WebsocketChannelSubscription) error {
	p.Websocket.RemoveSubscribedChannels(channels)
	return nil
}

// GetSubscriptions returns a copied list of subscriptions
func (p *Poloniex) GetSubscriptions() ([]wshandler.WebsocketChannelSubscription, error) {
	return p.Websocket.GetSubscriptions(), nil
}

// AuthenticateWebsocket sends an authentication message to the websocket
func (p *Poloniex) AuthenticateWebsocket() error {
	return common.ErrFunctionNotSupported
}
