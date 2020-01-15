package huobi

import (
	"errors"
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
func (h *HUOBI) GetDefaultConfig() (*config.ExchangeConfig, error) {
	h.SetDefaults()
	exchCfg := new(config.ExchangeConfig)
	exchCfg.Name = h.Name
	exchCfg.HTTPTimeout = exchange.DefaultHTTPTimeout
	exchCfg.BaseCurrencies = h.BaseCurrencies

	err := h.SetupDefaults(exchCfg)
	if err != nil {
		return nil, err
	}

	if h.Features.Supports.RESTCapabilities.AutoPairUpdates {
		err = h.UpdateTradablePairs(true)
		if err != nil {
			return nil, err
		}
	}

	return exchCfg, nil
}

// SetDefaults sets default values for the exchange
func (h *HUOBI) SetDefaults() {
	h.Name = "Huobi"
	h.Enabled = true
	h.Verbose = true
	h.API.CredentialsValidator.RequiresKey = true
	h.API.CredentialsValidator.RequiresSecret = true

	h.CurrencyPairs = currency.PairsManager{
		AssetTypes: asset.Items{
			asset.Spot,
		},

		UseGlobalFormat: true,
		RequestFormat: &currency.PairFormat{
			Uppercase: false,
		},
		ConfigFormat: &currency.PairFormat{
			Delimiter: "-",
			Uppercase: true,
		},
	}

	h.Features = exchange.Features{
		Supports: exchange.FeaturesSupported{
			REST:      true,
			Websocket: true,
			RESTCapabilities: protocol.Features{
				TickerBatching:    true,
				TickerFetching:    true,
				KlineFetching:     true,
				TradeFetching:     true,
				OrderbookFetching: true,
				AutoPairUpdates:   true,
				AccountInfo:       true,
				GetOrder:          true,
				GetOrders:         true,
				CancelOrders:      true,
				CancelOrder:       true,
				SubmitOrder:       true,
				CryptoWithdrawal:  true,
				TradeFee:          true,
			},
			WebsocketCapabilities: protocol.Features{
				KlineFetching:          true,
				OrderbookFetching:      true,
				TradeFetching:          true,
				Subscribe:              true,
				Unsubscribe:            true,
				AuthenticatedEndpoints: true,
				AccountInfo:            true,
				MessageCorrelation:     true,
				GetOrder:               true,
				GetOrders:              true,
			},
			WithdrawPermissions: exchange.AutoWithdrawCryptoWithSetup |
				exchange.NoFiatWithdrawals,
		},
		Enabled: exchange.FeaturesEnabled{
			AutoPairUpdates: true,
		},
	}

	h.Requester = request.New(h.Name,
		request.NewRateLimit(time.Second*10, huobiAuthRate),
		request.NewRateLimit(time.Second*10, huobiUnauthRate),
		common.NewHTTPClientWithTimeout(exchange.DefaultHTTPTimeout))

	h.API.Endpoints.URLDefault = huobiAPIURL
	h.API.Endpoints.URL = h.API.Endpoints.URLDefault
	h.API.Endpoints.WebsocketURL = wsMarketURL
	h.Websocket = wshandler.New()
	h.WebsocketResponseMaxLimit = exchange.DefaultWebsocketResponseMaxLimit
	h.WebsocketResponseCheckTimeout = exchange.DefaultWebsocketResponseCheckTimeout
	h.WebsocketOrderbookBufferLimit = exchange.DefaultWebsocketOrderbookBufferLimit
}

// Setup sets user configuration
func (h *HUOBI) Setup(exch *config.ExchangeConfig) error {
	if !exch.Enabled {
		h.SetEnabled(false)
		return nil
	}

	err := h.SetupDefaults(exch)
	if err != nil {
		return err
	}

	h.API.PEMKeySupport = exch.API.PEMKeySupport
	h.API.Credentials.PEMKey = exch.API.Credentials.PEMKey

	err = h.Websocket.Setup(
		&wshandler.WebsocketSetup{
			Enabled:                          exch.Features.Enabled.Websocket,
			Verbose:                          exch.Verbose,
			AuthenticatedWebsocketAPISupport: exch.API.AuthenticatedWebsocketSupport,
			WebsocketTimeout:                 exch.WebsocketTrafficTimeout,
			DefaultURL:                       wsMarketURL,
			ExchangeName:                     exch.Name,
			RunningURL:                       exch.API.Endpoints.WebsocketURL,
			Connector:                        h.WsConnect,
			Subscriber:                       h.Subscribe,
			UnSubscriber:                     h.Unsubscribe,
			Features:                         &h.Features.Supports.WebsocketCapabilities,
		})
	if err != nil {
		return err
	}

	h.WebsocketConn = &wshandler.WebsocketConnection{
		ExchangeName:         h.Name,
		URL:                  wsMarketURL,
		ProxyURL:             h.Websocket.GetProxyAddress(),
		Verbose:              h.Verbose,
		RateLimit:            rateLimit,
		ResponseCheckTimeout: exch.WebsocketResponseCheckTimeout,
		ResponseMaxLimit:     exch.WebsocketResponseMaxLimit,
	}
	h.AuthenticatedWebsocketConn = &wshandler.WebsocketConnection{
		ExchangeName:         h.Name,
		URL:                  wsAccountsOrdersURL,
		ProxyURL:             h.Websocket.GetProxyAddress(),
		Verbose:              h.Verbose,
		RateLimit:            rateLimit,
		ResponseCheckTimeout: exch.WebsocketResponseCheckTimeout,
		ResponseMaxLimit:     exch.WebsocketResponseMaxLimit,
	}

	h.Websocket.Orderbook.Setup(
		exch.WebsocketOrderbookBufferLimit,
		false,
		false,
		false,
		false,
		exch.Name)
	return nil
}

// Start starts the HUOBI go routine
func (h *HUOBI) Start(wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		h.Run()
		wg.Done()
	}()
}

// Run implements the HUOBI wrapper
func (h *HUOBI) Run() {
	if h.Verbose {
		log.Debugf(log.ExchangeSys,
			"%s Websocket: %s (url: %s).\n",
			h.Name,
			common.IsEnabled(h.Websocket.IsEnabled()),
			wsMarketURL)
		h.PrintEnabledPairs()
	}

	var forceUpdate bool
	if common.StringDataContains(h.GetEnabledPairs(asset.Spot).Strings(), currency.CNY.String()) ||
		common.StringDataContains(h.GetAvailablePairs(asset.Spot).Strings(), currency.CNY.String()) {
		forceUpdate = true
	}

	if common.StringDataContains(h.BaseCurrencies.Strings(), currency.CNY.String()) {
		cfg := config.GetConfig()
		exchCfg, err := cfg.GetExchangeConfig(h.Name)
		if err != nil {
			log.Errorf(log.ExchangeSys,
				"%s failed to get exchange config. %s\n",
				h.Name,
				err)
			return
		}
		exchCfg.BaseCurrencies = currency.Currencies{currency.USD}
		h.BaseCurrencies = currency.Currencies{currency.USD}
	}

	if forceUpdate {
		enabledPairs := currency.Pairs{currency.Pair{
			Base:      currency.BTC.Lower(),
			Quote:     currency.USDT.Lower(),
			Delimiter: h.GetPairFormat(asset.Spot, false).Delimiter,
		},
		}
		log.Warn(log.ExchangeSys,
			"Available and enabled pairs for Huobi reset due to config upgrade, please enable the ones you would like again")

		err := h.UpdatePairs(enabledPairs, asset.Spot, true, true)
		if err != nil {
			log.Errorf(log.ExchangeSys,
				"%s Failed to update enabled currencies.\n",
				h.Name)
		}
	}

	if !h.GetEnabledFeatures().AutoPairUpdates && !forceUpdate {
		return
	}

	err := h.UpdateTradablePairs(forceUpdate)
	if err != nil {
		log.Errorf(log.ExchangeSys,
			"%s failed to update tradable pairs. Err: %s",
			h.Name,
			err)
	}
}

// FetchTradablePairs returns a list of the exchanges tradable pairs
func (h *HUOBI) FetchTradablePairs(asset asset.Item) ([]string, error) {
	symbols, err := h.GetSymbols()
	if err != nil {
		return nil, err
	}

	var pairs []string
	for x := range symbols {
		if symbols[x].State != "online" {
			continue
		}
		pairs = append(pairs, symbols[x].BaseCurrency+
			h.GetPairFormat(asset, false).Delimiter+
			symbols[x].QuoteCurrency)
	}

	return pairs, nil
}

// UpdateTradablePairs updates the exchanges available pairs and stores
// them in the exchanges config
func (h *HUOBI) UpdateTradablePairs(forceUpdate bool) error {
	pairs, err := h.FetchTradablePairs(asset.Spot)
	if err != nil {
		return err
	}

	return h.UpdatePairs(currency.NewPairsFromStrings(pairs),
		asset.Spot,
		false,
		forceUpdate)
}

// UpdateTicker updates and returns the ticker for a currency pair
func (h *HUOBI) UpdateTicker(p currency.Pair, assetType asset.Item) (*ticker.Price, error) {
	tickerPrice := new(ticker.Price)
	tickers, err := h.GetTickers()
	if err != nil {
		return tickerPrice, err
	}
	pairs := h.GetEnabledPairs(assetType)
	for i := range pairs {
		for j := range tickers.Data {
			pairFmt := h.FormatExchangeCurrency(pairs[i], assetType).String()
			if pairFmt != tickers.Data[j].Symbol {
				continue
			}
			tickerPrice := &ticker.Price{
				High:   tickers.Data[j].High,
				Low:    tickers.Data[j].Low,
				Volume: tickers.Data[j].Volume,
				Open:   tickers.Data[j].Open,
				Close:  tickers.Data[j].Close,
				Pair:   pairs[i],
			}
			err = ticker.ProcessTicker(h.Name, tickerPrice, assetType)
			if err != nil {
				log.Error(log.Ticker, err)
			}
		}
	}

	return ticker.GetTicker(h.Name, p, assetType)
}

// FetchTicker returns the ticker for a currency pair
func (h *HUOBI) FetchTicker(p currency.Pair, assetType asset.Item) (*ticker.Price, error) {
	tickerNew, err := ticker.GetTicker(h.Name, p, assetType)
	if err != nil {
		return h.UpdateTicker(p, assetType)
	}
	return tickerNew, nil
}

// FetchOrderbook returns orderbook base on the currency pair
func (h *HUOBI) FetchOrderbook(p currency.Pair, assetType asset.Item) (*orderbook.Base, error) {
	ob, err := orderbook.Get(h.Name, p, assetType)
	if err != nil {
		return h.UpdateOrderbook(p, assetType)
	}
	return ob, nil
}

// UpdateOrderbook updates and returns the orderbook for a currency pair
func (h *HUOBI) UpdateOrderbook(p currency.Pair, assetType asset.Item) (*orderbook.Base, error) {
	orderBook := new(orderbook.Base)
	orderbookNew, err := h.GetDepth(OrderBookDataRequestParams{
		Symbol: h.FormatExchangeCurrency(p, assetType).String(),
		Type:   OrderBookDataRequestParamsTypeStep0,
	})
	if err != nil {
		return orderBook, err
	}

	for x := range orderbookNew.Bids {
		orderBook.Bids = append(orderBook.Bids, orderbook.Item{
			Amount: orderbookNew.Bids[x][1],
			Price:  orderbookNew.Bids[x][0],
		})
	}

	for x := range orderbookNew.Asks {
		orderBook.Asks = append(orderBook.Asks, orderbook.Item{
			Amount: orderbookNew.Asks[x][1],
			Price:  orderbookNew.Asks[x][0],
		})
	}

	orderBook.Pair = p
	orderBook.ExchangeName = h.Name
	orderBook.AssetType = assetType

	err = orderBook.Process()
	if err != nil {
		return orderBook, err
	}

	return orderbook.Get(h.Name, p, assetType)
}

// GetAccountID returns the account ID for trades
func (h *HUOBI) GetAccountID() ([]Account, error) {
	acc, err := h.GetAccounts()
	if err != nil {
		return nil, err
	}

	if len(acc) < 1 {
		return nil, errors.New("no account returned")
	}

	return acc, nil
}

// GetAccountInfo retrieves balances for all enabled currencies for the
// HUOBI exchange - to-do
func (h *HUOBI) GetAccountInfo() (exchange.AccountInfo, error) {
	var info exchange.AccountInfo
	info.Exchange = h.Name
	if h.Websocket.CanUseAuthenticatedWebsocketForWrapper() {
		resp, err := h.wsGetAccountsList()
		if err != nil {
			return info, err
		}
		var currencyDetails []exchange.AccountCurrencyInfo
		for i := range resp.Data {
			if len(resp.Data[i].List) == 0 {
				continue
			}
			currData := exchange.AccountCurrencyInfo{
				CurrencyName: currency.NewCode(resp.Data[i].List[0].Currency),
				TotalValue:   resp.Data[i].List[0].Balance,
			}
			if len(resp.Data[i].List) > 1 && resp.Data[i].List[1].Type == "frozen" {
				currData.Hold = resp.Data[i].List[1].Balance
			}
			currencyDetails = append(currencyDetails, currData)
		}
		var acc exchange.Account
		acc.Currencies = currencyDetails
		info.Accounts = append(info.Accounts, acc)
	} else {
		accounts, err := h.GetAccountID()
		if err != nil {
			return info, err
		}
		for i := range accounts {
			var acc exchange.Account
			acc.ID = strconv.FormatInt(accounts[i].ID, 10)
			balances, err := h.GetAccountBalance(acc.ID)
			if err != nil {
				return info, err
			}

			var currencyDetails []exchange.AccountCurrencyInfo
			for j := range balances {
				var frozen bool
				if balances[j].Type == "frozen" {
					frozen = true
				}

				var updated bool
				for i := range currencyDetails {
					if currencyDetails[i].CurrencyName == currency.NewCode(balances[j].Currency) {
						if frozen {
							currencyDetails[i].Hold = balances[j].Balance
						} else {
							currencyDetails[i].TotalValue = balances[j].Balance
						}
						updated = true
					}
				}

				if updated {
					continue
				}

				if frozen {
					currencyDetails = append(currencyDetails,
						exchange.AccountCurrencyInfo{
							CurrencyName: currency.NewCode(balances[j].Currency),
							Hold:         balances[j].Balance,
						})
				} else {
					currencyDetails = append(currencyDetails,
						exchange.AccountCurrencyInfo{
							CurrencyName: currency.NewCode(balances[j].Currency),
							TotalValue:   balances[j].Balance,
						})
				}
			}

			acc.Currencies = currencyDetails
			info.Accounts = append(info.Accounts, acc)
		}
	}
	return info, nil
}

// GetFundingHistory returns funding history, deposits and
// withdrawals
func (h *HUOBI) GetFundingHistory() ([]exchange.FundHistory, error) {
	return nil, common.ErrFunctionNotSupported
}

// GetExchangeHistory returns historic trade data since exchange opening.
func (h *HUOBI) GetExchangeHistory(p currency.Pair, assetType asset.Item) ([]exchange.TradeHistory, error) {
	return nil, common.ErrNotYetImplemented
}

// SubmitOrder submits a new order
func (h *HUOBI) SubmitOrder(s *order.Submit) (order.SubmitResponse, error) {
	var submitOrderResponse order.SubmitResponse
	if err := s.Validate(); err != nil {
		return submitOrderResponse, err
	}

	accountID, err := strconv.ParseInt(s.ClientID, 10, 64)
	if err != nil {
		return submitOrderResponse, err
	}

	var formattedType SpotNewOrderRequestParamsType
	var params = SpotNewOrderRequestParams{
		Amount:    s.Amount,
		Source:    "api",
		Symbol:    s.Pair.Lower().String(),
		AccountID: int(accountID),
	}

	switch {
	case s.OrderSide == order.Buy && s.OrderType == order.Market:
		formattedType = SpotNewOrderRequestTypeBuyMarket
	case s.OrderSide == order.Sell && s.OrderType == order.Market:
		formattedType = SpotNewOrderRequestTypeSellMarket
	case s.OrderSide == order.Buy && s.OrderType == order.Limit:
		formattedType = SpotNewOrderRequestTypeBuyLimit
		params.Price = s.Price
	case s.OrderSide == order.Sell && s.OrderType == order.Limit:
		formattedType = SpotNewOrderRequestTypeSellLimit
		params.Price = s.Price
	}

	params.Type = formattedType
	response, err := h.SpotNewOrder(params)
	if err != nil {
		return submitOrderResponse, err
	}
	if response > 0 {
		submitOrderResponse.OrderID = strconv.FormatInt(response, 10)
	}

	submitOrderResponse.IsOrderPlaced = true
	if s.OrderType == order.Market {
		submitOrderResponse.FullyMatched = true
	}
	return submitOrderResponse, nil
}

// ModifyOrder will allow of changing orderbook placement and limit to
// market conversion
func (h *HUOBI) ModifyOrder(action *order.Modify) (string, error) {
	return "", common.ErrFunctionNotSupported
}

// CancelOrder cancels an order by its corresponding ID number
func (h *HUOBI) CancelOrder(order *order.Cancel) error {
	orderIDInt, err := strconv.ParseInt(order.OrderID, 10, 64)
	if err != nil {
		return err
	}

	_, err = h.CancelExistingOrder(orderIDInt)
	return err
}

// CancelAllOrders cancels all orders associated with a currency pair
func (h *HUOBI) CancelAllOrders(orderCancellation *order.Cancel) (order.CancelAllResponse, error) {
	var cancelAllOrdersResponse order.CancelAllResponse
	enabledPairs := h.GetEnabledPairs(asset.Spot)
	for i := range enabledPairs {
		resp, err := h.CancelOpenOrdersBatch(orderCancellation.AccountID,
			h.FormatExchangeCurrency(enabledPairs[i], asset.Spot).String())
		if err != nil {
			return cancelAllOrdersResponse, err
		}

		if resp.Data.FailedCount > 0 {
			return cancelAllOrdersResponse,
				fmt.Errorf("%v orders failed to cancel",
					resp.Data.FailedCount)
		}

		if resp.Status == "error" {
			return cancelAllOrdersResponse, errors.New(resp.ErrorMessage)
		}
	}

	return cancelAllOrdersResponse, nil
}

// GetOrderInfo returns information on a current open order
func (h *HUOBI) GetOrderInfo(orderID string) (order.Detail, error) {
	var orderDetail order.Detail
	var respData *OrderInfo
	if h.Websocket.CanUseAuthenticatedWebsocketForWrapper() {
		resp, err := h.wsGetOrderDetails(orderID)
		if err != nil {
			return orderDetail, err
		}
		respData = &resp.Data
	} else {
		oID, err := strconv.ParseInt(orderID, 10, 64)
		if err != nil {
			return orderDetail, err
		}
		resp, err := h.GetOrder(oID)
		if err != nil {
			return orderDetail, err
		}
		respData = &resp
	}
	if respData.ID == 0 {
		return orderDetail, fmt.Errorf("%s - order not found for orderid %s", h.Name, orderID)
	}

	typeDetails := strings.Split(respData.Type, "-")
	orderSide, err := order.StringToOrderSide(typeDetails[0])
	if err != nil {
		return orderDetail, err
	}
	orderType, err := order.StringToOrderType(typeDetails[1])
	if err != nil {
		return orderDetail, err
	}
	orderStatus, err := order.StringToOrderStatus(respData.State)
	if err != nil {
		return orderDetail, err
	}
	orderDetail = order.Detail{
		Exchange:       h.Name,
		ID:             strconv.FormatInt(respData.ID, 10),
		AccountID:      strconv.FormatInt(respData.AccountID, 10),
		CurrencyPair:   currency.NewPairFromString(respData.Symbol),
		OrderType:      orderType,
		OrderSide:      orderSide,
		OrderDate:      time.Unix(0, respData.CreatedAt*int64(time.Millisecond)),
		Status:         orderStatus,
		Price:          respData.Price,
		Amount:         respData.Amount,
		ExecutedAmount: respData.FilledAmount,
		Fee:            respData.FilledFees,
	}
	return orderDetail, nil
}

// GetDepositAddress returns a deposit address for a specified currency
func (h *HUOBI) GetDepositAddress(cryptocurrency currency.Code, accountID string) (string, error) {
	return "", common.ErrFunctionNotSupported
}

// WithdrawCryptocurrencyFunds returns a withdrawal ID when a withdrawal is
// submitted
func (h *HUOBI) WithdrawCryptocurrencyFunds(withdrawRequest *withdraw.CryptoRequest) (string, error) {
	resp, err := h.Withdraw(withdrawRequest.Currency, withdrawRequest.Address, withdrawRequest.AddressTag, withdrawRequest.Amount, withdrawRequest.FeeAmount)
	return strconv.FormatInt(resp, 10), err
}

// WithdrawFiatFunds returns a withdrawal ID when a
// withdrawal is submitted
func (h *HUOBI) WithdrawFiatFunds(withdrawRequest *withdraw.FiatRequest) (string, error) {
	return "", common.ErrFunctionNotSupported
}

// WithdrawFiatFundsToInternationalBank returns a withdrawal ID when a
// withdrawal is submitted
func (h *HUOBI) WithdrawFiatFundsToInternationalBank(withdrawRequest *withdraw.FiatRequest) (string, error) {
	return "", common.ErrFunctionNotSupported
}

// GetWebsocket returns a pointer to the exchange websocket
func (h *HUOBI) GetWebsocket() (*wshandler.Websocket, error) {
	return h.Websocket, nil
}

// GetFeeByType returns an estimate of fee based on type of transaction
func (h *HUOBI) GetFeeByType(feeBuilder *exchange.FeeBuilder) (float64, error) {
	if !h.AllowAuthenticatedRequest() && // Todo check connection status
		feeBuilder.FeeType == exchange.CryptocurrencyTradeFee {
		feeBuilder.FeeType = exchange.OfflineTradeFee
	}
	return h.GetFee(feeBuilder)
}

// GetActiveOrders retrieves any orders that are active/open
func (h *HUOBI) GetActiveOrders(req *order.GetOrdersRequest) ([]order.Detail, error) {
	if len(req.Currencies) == 0 {
		return nil, errors.New("currency must be supplied")
	}

	side := ""
	if req.OrderSide == order.AnySide || req.OrderSide == "" {
		side = ""
	} else if req.OrderSide == order.Sell {
		side = req.OrderSide.Lower()
	}

	var orders []order.Detail

	if h.Websocket.CanUseAuthenticatedWebsocketForWrapper() {
		for i := range req.Currencies {
			resp, err := h.wsGetOrdersList(-1, req.Currencies[i])
			if err != nil {
				return orders, err
			}
			for j := range resp.Data {
				sideData := strings.Split(resp.Data[j].OrderState, "-")
				side = sideData[0]
				orderSide, err := order.StringToOrderSide(side)
				if err != nil {
					return orders, err
				}
				orderType, err := order.StringToOrderType(sideData[1])
				if err != nil {
					return orders, err
				}
				orderStatus, err := order.StringToOrderStatus(resp.Data[j].OrderState)
				if err != nil {
					return orders, err
				}
				orders = append(orders, order.Detail{
					Exchange:        h.Name,
					AccountID:       strconv.FormatInt(resp.Data[j].AccountID, 10),
					ID:              strconv.FormatInt(resp.Data[j].OrderID, 10),
					CurrencyPair:    req.Currencies[i],
					OrderType:       orderType,
					OrderSide:       orderSide,
					OrderDate:       time.Unix(0, resp.Data[j].CreatedAt*int64(time.Millisecond)),
					Status:          orderStatus,
					Price:           resp.Data[j].Price,
					Amount:          resp.Data[j].OrderAmount,
					ExecutedAmount:  resp.Data[j].FilledAmount,
					RemainingAmount: resp.Data[j].UnfilledAmount,
					Fee:             resp.Data[j].FilledFees,
				})
			}
		}
	} else {
		for i := range req.Currencies {
			resp, err := h.GetOpenOrders(h.API.Credentials.ClientID,
				req.Currencies[i].Lower().String(),
				side,
				500)
			if err != nil {
				return nil, err
			}

			for i := range resp {
				orderDetail := order.Detail{
					ID:             strconv.FormatInt(resp[i].ID, 10),
					Price:          resp[i].Price,
					Amount:         resp[i].Amount,
					CurrencyPair:   req.Currencies[i],
					Exchange:       h.Name,
					ExecutedAmount: resp[i].FilledAmount,
					OrderDate:      time.Unix(0, resp[i].CreatedAt*int64(time.Millisecond)),
					Status:         order.Status(resp[i].State),
					AccountID:      strconv.FormatInt(resp[i].AccountID, 10),
					Fee:            resp[i].FilledFees,
				}

				setOrderSideAndType(resp[i].Type, &orderDetail)

				orders = append(orders, orderDetail)
			}
		}
	}

	order.FilterOrdersByTickRange(&orders, req.StartTicks, req.EndTicks)
	return orders, nil
}

// GetOrderHistory retrieves account order information
// Can Limit response to specific order status
func (h *HUOBI) GetOrderHistory(req *order.GetOrdersRequest) ([]order.Detail, error) {
	if len(req.Currencies) == 0 {
		return nil, errors.New("currency must be supplied")
	}

	states := "partial-canceled,filled,canceled"
	var orders []order.Detail
	for i := range req.Currencies {
		resp, err := h.GetOrders(req.Currencies[i].Lower().String(),
			"",
			"",
			"",
			states,
			"",
			"",
			"")
		if err != nil {
			return nil, err
		}

		for i := range resp {
			orderDetail := order.Detail{
				ID:             strconv.FormatInt(resp[i].ID, 10),
				Price:          resp[i].Price,
				Amount:         resp[i].Amount,
				CurrencyPair:   req.Currencies[i],
				Exchange:       h.Name,
				ExecutedAmount: resp[i].FilledAmount,
				OrderDate:      time.Unix(0, resp[i].CreatedAt*int64(time.Millisecond)),
				Status:         order.Status(resp[i].State),
				AccountID:      strconv.FormatInt(resp[i].AccountID, 10),
				Fee:            resp[i].FilledFees,
			}

			setOrderSideAndType(resp[i].Type, &orderDetail)

			orders = append(orders, orderDetail)
		}
	}

	order.FilterOrdersByTickRange(&orders, req.StartTicks, req.EndTicks)
	return orders, nil
}

func setOrderSideAndType(requestType string, orderDetail *order.Detail) {
	switch SpotNewOrderRequestParamsType(requestType) {
	case SpotNewOrderRequestTypeBuyMarket:
		orderDetail.OrderSide = order.Buy
		orderDetail.OrderType = order.Market
	case SpotNewOrderRequestTypeSellMarket:
		orderDetail.OrderSide = order.Sell
		orderDetail.OrderType = order.Market
	case SpotNewOrderRequestTypeBuyLimit:
		orderDetail.OrderSide = order.Buy
		orderDetail.OrderType = order.Limit
	case SpotNewOrderRequestTypeSellLimit:
		orderDetail.OrderSide = order.Sell
		orderDetail.OrderType = order.Limit
	}
}

// SubscribeToWebsocketChannels appends to ChannelsToSubscribe
// which lets websocket.manageSubscriptions handle subscribing
func (h *HUOBI) SubscribeToWebsocketChannels(channels []wshandler.WebsocketChannelSubscription) error {
	h.Websocket.SubscribeToChannels(channels)
	return nil
}

// UnsubscribeToWebsocketChannels removes from ChannelsToSubscribe
// which lets websocket.manageSubscriptions handle unsubscribing
func (h *HUOBI) UnsubscribeToWebsocketChannels(channels []wshandler.WebsocketChannelSubscription) error {
	h.Websocket.RemoveSubscribedChannels(channels)
	return nil
}

// GetSubscriptions returns a copied list of subscriptions
func (h *HUOBI) GetSubscriptions() ([]wshandler.WebsocketChannelSubscription, error) {
	return h.Websocket.GetSubscriptions(), nil
}

// AuthenticateWebsocket sends an authentication message to the websocket
func (h *HUOBI) AuthenticateWebsocket() error {
	return h.wsLogin()
}
