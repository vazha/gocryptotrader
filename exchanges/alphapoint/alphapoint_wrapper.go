package alphapoint

import (
	"errors"
	"strconv"
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
)

// GetDefaultConfig returns a default exchange config for Alphapoint
func (a *Alphapoint) GetDefaultConfig() (*config.ExchangeConfig, error) {
	return nil, common.ErrFunctionNotSupported
}

// SetDefaults sets current default settings
func (a *Alphapoint) SetDefaults() {
	a.Name = "Alphapoint"
	a.Enabled = true
	a.Verbose = true
	a.API.Endpoints.URL = alphapointDefaultAPIURL
	a.API.Endpoints.WebsocketURL = alphapointDefaultWebsocketURL
	a.API.CredentialsValidator.RequiresKey = true
	a.API.CredentialsValidator.RequiresSecret = true

	a.CurrencyPairs.AssetTypes = asset.Items{
		asset.Spot,
	}

	a.Features = exchange.Features{
		Supports: exchange.FeaturesSupported{
			REST:      true,
			Websocket: true,
			RESTCapabilities: protocol.Features{
				AccountInfo:       true,
				TickerFetching:    true,
				TradeFetching:     true,
				OrderbookFetching: true,
				GetOrders:         true,
				CancelOrder:       true,
				CancelOrders:      true,
				SubmitOrder:       true,
				ModifyOrder:       true,
				UserTradeHistory:  true,
				CryptoDeposit:     true,
				CryptoWithdrawal:  true,
				TradeFee:          true,
			},

			WebsocketCapabilities: protocol.Features{
				AccountInfo: true,
			},

			WithdrawPermissions: exchange.WithdrawCryptoWith2FA |
				exchange.AutoWithdrawCryptoWithAPIPermission |
				exchange.NoFiatWithdrawals,
		},
	}

	a.Requester = request.New(a.Name,
		request.NewRateLimit(time.Minute*10, alphapointAuthRate),
		request.NewRateLimit(time.Minute*10, alphapointUnauthRate),
		common.NewHTTPClientWithTimeout(exchange.DefaultHTTPTimeout))
}

// FetchTradablePairs returns a list of the exchanges tradable pairs
func (a *Alphapoint) FetchTradablePairs(asset asset.Item) ([]string, error) {
	return nil, common.ErrFunctionNotSupported
}

// UpdateTradablePairs updates the exchanges available pairs and stores
// them in the exchanges config
func (a *Alphapoint) UpdateTradablePairs(forceUpdate bool) error {
	return common.ErrFunctionNotSupported
}

// GetAccountInfo retrieves balances for all enabled currencies on the
// Alphapoint exchange
func (a *Alphapoint) GetAccountInfo() (exchange.AccountInfo, error) {
	var response exchange.AccountInfo
	response.Exchange = a.Name
	account, err := a.GetAccountInformation()
	if err != nil {
		return response, err
	}

	var currencies []exchange.AccountCurrencyInfo
	for i := 0; i < len(account.Currencies); i++ {
		var exchangeCurrency exchange.AccountCurrencyInfo
		exchangeCurrency.CurrencyName = currency.NewCode(account.Currencies[i].Name)
		exchangeCurrency.TotalValue = float64(account.Currencies[i].Balance)
		exchangeCurrency.Hold = float64(account.Currencies[i].Hold)

		currencies = append(currencies, exchangeCurrency)
	}

	response.Accounts = append(response.Accounts, exchange.Account{
		Currencies: currencies,
	})

	return response, nil
}

// UpdateTicker updates and returns the ticker for a currency pair
func (a *Alphapoint) UpdateTicker(p currency.Pair, assetType asset.Item) (*ticker.Price, error) {
	tickerPrice := new(ticker.Price)
	tick, err := a.GetTicker(p.String())
	if err != nil {
		return tickerPrice, err
	}

	tickerPrice.Pair = p
	tickerPrice.Ask = tick.Ask
	tickerPrice.Bid = tick.Bid
	tickerPrice.Low = tick.Low
	tickerPrice.High = tick.High
	tickerPrice.Volume = tick.Volume
	tickerPrice.Last = tick.Last

	err = ticker.ProcessTicker(a.Name, tickerPrice, assetType)
	if err != nil {
		return tickerPrice, err
	}

	return ticker.GetTicker(a.Name, p, assetType)
}

// FetchTicker returns the ticker for a currency pair
func (a *Alphapoint) FetchTicker(p currency.Pair, assetType asset.Item) (*ticker.Price, error) {
	tick, err := ticker.GetTicker(a.Name, p, assetType)
	if err != nil {
		return a.UpdateTicker(p, assetType)
	}
	return tick, nil
}

// UpdateOrderbook updates and returns the orderbook for a currency pair
func (a *Alphapoint) UpdateOrderbook(p currency.Pair, assetType asset.Item) (*orderbook.Base, error) {
	orderBook := new(orderbook.Base)
	orderbookNew, err := a.GetOrderbook(p.String())
	if err != nil {
		return orderBook, err
	}

	for x := range orderbookNew.Bids {
		orderBook.Bids = append(orderBook.Bids, orderbook.Item{
			Amount: orderbookNew.Bids[x].Quantity,
			Price:  orderbookNew.Bids[x].Price,
		})
	}

	for x := range orderbookNew.Asks {
		orderBook.Asks = append(orderBook.Asks, orderbook.Item{
			Amount: orderbookNew.Asks[x].Quantity,
			Price:  orderbookNew.Asks[x].Price,
		})
	}

	orderBook.Pair = p
	orderBook.ExchangeName = a.Name
	orderBook.AssetType = assetType

	err = orderBook.Process()
	if err != nil {
		return orderBook, err
	}

	return orderbook.Get(a.Name, p, assetType)
}

// FetchOrderbook returns the orderbook for a currency pair
func (a *Alphapoint) FetchOrderbook(p currency.Pair, assetType asset.Item) (*orderbook.Base, error) {
	ob, err := orderbook.Get(a.Name, p, assetType)
	if err != nil {
		return a.UpdateOrderbook(p, assetType)
	}
	return ob, nil
}

// GetFundingHistory returns funding history, deposits and
// withdrawals
func (a *Alphapoint) GetFundingHistory() ([]exchange.FundHistory, error) {
	// https://alphapoint.github.io/slate/#generatetreasuryactivityreport
	return nil, common.ErrNotYetImplemented
}

// GetExchangeHistory returns historic trade data since exchange opening.
func (a *Alphapoint) GetExchangeHistory(p currency.Pair, assetType asset.Item) ([]exchange.TradeHistory, error) {
	return nil, common.ErrNotYetImplemented
}

// SubmitOrder submits a new order and returns a true value when
// successfully submitted
func (a *Alphapoint) SubmitOrder(s *order.Submit) (order.SubmitResponse, error) {
	var submitOrderResponse order.SubmitResponse
	if err := s.Validate(); err != nil {
		return submitOrderResponse, err
	}

	response, err := a.CreateOrder(s.Pair.String(),
		s.OrderSide.String(),
		s.OrderSide.String(),
		s.Amount,
		s.Price)
	if err != nil {
		return submitOrderResponse, err
	}
	if response > 0 {
		submitOrderResponse.OrderID = strconv.FormatInt(response, 10)
	}
	if s.OrderType == order.Market {
		submitOrderResponse.FullyMatched = true
	}
	submitOrderResponse.IsOrderPlaced = true

	return submitOrderResponse, nil
}

// ModifyOrder will allow of changing orderbook placement and limit to
// market conversion
func (a *Alphapoint) ModifyOrder(_ *order.Modify) (string, error) {
	return "", common.ErrNotYetImplemented
}

// CancelOrder cancels an order by its corresponding ID number
func (a *Alphapoint) CancelOrder(order *order.Cancel) error {
	orderIDInt, err := strconv.ParseInt(order.OrderID, 10, 64)
	if err != nil {
		return err
	}
	_, err = a.CancelExistingOrder(orderIDInt, order.AccountID)
	return err
}

// CancelAllOrders cancels all orders for a given account
func (a *Alphapoint) CancelAllOrders(orderCancellation *order.Cancel) (order.CancelAllResponse, error) {
	return order.CancelAllResponse{},
		a.CancelAllExistingOrders(orderCancellation.AccountID)
}

// GetOrderInfo returns information on a current open order
func (a *Alphapoint) GetOrderInfo(orderID string) (float64, error) {
	orders, err := a.GetOrders()
	if err != nil {
		return 0, err
	}

	for x := range orders {
		for y := range orders[x].OpenOrders {
			if strconv.Itoa(orders[x].OpenOrders[y].ServerOrderID) == orderID {
				return orders[x].OpenOrders[y].QtyRemaining, nil
			}
		}
	}
	return 0, errors.New("order not found")
}

// GetDepositAddress returns a deposit address for a specified currency
func (a *Alphapoint) GetDepositAddress(cryptocurrency currency.Code, _ string) (string, error) {
	addreses, err := a.GetDepositAddresses()
	if err != nil {
		return "", err
	}

	for x := range addreses {
		if addreses[x].Name == cryptocurrency.String() {
			return addreses[x].DepositAddress, nil
		}
	}
	return "", errors.New("associated currency address not found")
}

// WithdrawCryptocurrencyFunds returns a withdrawal ID when a withdrawal is
// submitted
func (a *Alphapoint) WithdrawCryptocurrencyFunds(withdrawRequest *withdraw.CryptoRequest) (string, error) {
	return "", common.ErrNotYetImplemented
}

// WithdrawFiatFunds returns a withdrawal ID when a withdrawal is submitted
func (a *Alphapoint) WithdrawFiatFunds(withdrawRequest *withdraw.FiatRequest) (string, error) {
	return "", common.ErrNotYetImplemented
}

// WithdrawFiatFundsToInternationalBank returns a withdrawal ID when a withdrawal is
// submitted
func (a *Alphapoint) WithdrawFiatFundsToInternationalBank(withdrawRequest *withdraw.FiatRequest) (string, error) {
	return "", common.ErrNotYetImplemented
}

// GetWebsocket returns a pointer to the exchange websocket
func (a *Alphapoint) GetWebsocket() (*wshandler.Websocket, error) {
	return nil, common.ErrNotYetImplemented
}

// GetFeeByType returns an estimate of fee based on type of transaction
func (a *Alphapoint) GetFeeByType(feeBuilder *exchange.FeeBuilder) (float64, error) {
	return 0, common.ErrFunctionNotSupported
}

// GetActiveOrders retrieves any orders that are active/open
// This function is not concurrency safe due to orderSide/orderType maps
func (a *Alphapoint) GetActiveOrders(req *order.GetOrdersRequest) ([]order.Detail, error) {
	resp, err := a.GetOrders()
	if err != nil {
		return nil, err
	}

	var orders []order.Detail
	for x := range resp {
		for y := range resp[x].OpenOrders {
			if resp[x].OpenOrders[y].State != 1 {
				continue
			}

			orderDetail := order.Detail{
				Amount:          resp[x].OpenOrders[y].QtyTotal,
				Exchange:        a.Name,
				AccountID:       strconv.FormatInt(int64(resp[x].OpenOrders[y].AccountID), 10),
				ID:              strconv.FormatInt(int64(resp[x].OpenOrders[y].ServerOrderID), 10),
				Price:           resp[x].OpenOrders[y].Price,
				RemainingAmount: resp[x].OpenOrders[y].QtyRemaining,
			}

			orderDetail.OrderSide = orderSideMap[resp[x].OpenOrders[y].Side]
			orderDetail.OrderDate = time.Unix(resp[x].OpenOrders[y].ReceiveTime, 0)
			orderDetail.OrderType = orderTypeMap[resp[x].OpenOrders[y].OrderType]
			if orderDetail.OrderType == "" {
				orderDetail.OrderType = order.Unknown
			}

			orders = append(orders, orderDetail)
		}
	}

	order.FilterOrdersByType(&orders, req.OrderType)
	order.FilterOrdersBySide(&orders, req.OrderSide)
	order.FilterOrdersByTickRange(&orders, req.StartTicks, req.EndTicks)
	return orders, nil
}

// GetOrderHistory retrieves account order information
// Can Limit response to specific order status
// This function is not concurrency safe due to orderSide/orderType maps
func (a *Alphapoint) GetOrderHistory(req *order.GetOrdersRequest) ([]order.Detail, error) {
	resp, err := a.GetOrders()
	if err != nil {
		return nil, err
	}

	var orders []order.Detail
	for x := range resp {
		for y := range resp[x].OpenOrders {
			if resp[x].OpenOrders[y].State == 1 {
				continue
			}

			orderDetail := order.Detail{
				Amount:          resp[x].OpenOrders[y].QtyTotal,
				AccountID:       strconv.FormatInt(int64(resp[x].OpenOrders[y].AccountID), 10),
				Exchange:        a.Name,
				ID:              strconv.FormatInt(int64(resp[x].OpenOrders[y].ServerOrderID), 10),
				Price:           resp[x].OpenOrders[y].Price,
				RemainingAmount: resp[x].OpenOrders[y].QtyRemaining,
			}

			orderDetail.OrderSide = orderSideMap[resp[x].OpenOrders[y].Side]
			orderDetail.OrderDate = time.Unix(resp[x].OpenOrders[y].ReceiveTime, 0)
			orderDetail.OrderType = orderTypeMap[resp[x].OpenOrders[y].OrderType]
			if orderDetail.OrderType == "" {
				orderDetail.OrderType = order.Unknown
			}

			orders = append(orders, orderDetail)
		}
	}

	order.FilterOrdersByType(&orders, req.OrderType)
	order.FilterOrdersBySide(&orders, req.OrderSide)
	order.FilterOrdersByTickRange(&orders, req.StartTicks, req.EndTicks)
	return orders, nil
}

// SubscribeToWebsocketChannels appends to ChannelsToSubscribe
// which lets websocket.manageSubscriptions handle subscribing
func (a *Alphapoint) SubscribeToWebsocketChannels(channels []wshandler.WebsocketChannelSubscription) error {
	return common.ErrFunctionNotSupported
}

// UnsubscribeToWebsocketChannels removes from ChannelsToSubscribe
// which lets websocket.manageSubscriptions handle unsubscribing
func (a *Alphapoint) UnsubscribeToWebsocketChannels(channels []wshandler.WebsocketChannelSubscription) error {
	return common.ErrFunctionNotSupported
}

// GetSubscriptions returns a copied list of subscriptions
func (a *Alphapoint) GetSubscriptions() ([]wshandler.WebsocketChannelSubscription, error) {
	return nil, common.ErrFunctionNotSupported
}

// AuthenticateWebsocket sends an authentication message to the websocket
func (a *Alphapoint) AuthenticateWebsocket() error {
	return common.ErrFunctionNotSupported
}
