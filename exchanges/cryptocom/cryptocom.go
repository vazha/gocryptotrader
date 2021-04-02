package cryptocom

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/vazha/gocryptotrader/common"
	"github.com/vazha/gocryptotrader/common/crypto"
	"github.com/vazha/gocryptotrader/currency"
	exchange "github.com/vazha/gocryptotrader/exchanges"
	"github.com/vazha/gocryptotrader/exchanges/asset"
	"github.com/vazha/gocryptotrader/exchanges/request"
	"github.com/vazha/gocryptotrader/log"
)

// Cryptocom is the overarching type across this package
type Cryptocom struct {
	exchange.Base
}

const (
	cryptocomAPIURL         = "https://uat-api.3ona.co" //"https://api.crypto.com"
	cryptocomSPOTAPIPath    = "/v2/"
	cryptocomFuturesAPIPath = "/v2/"

	// Public endpoints
	cryptocomMarketOverview = "public/get-instruments"
	cryptocomOrderbook      = "public/get-book"
	cryptocomTrades         = "public/get-trades"
	cryptocomTime           = "time"
	cryptocomOHLCV          = "ohlcv"
	cryptocomPrice          = "public/get-ticker"
	cryptocomFuturesFunding = "funding_history"

	// Authenticated endpoints
	cryptocomWallet           = "private/get-account-summary"
	cryptocomWalletHistory    = "private/get-withdrawal-history"
	cryptocomWalletAddress    = "user/wallet/address"
	cryptocomWalletWithdrawal = "private/create-withdrawal"
	cryptocomExchangeHistory  = "user/trade_history"
	cryptocomUserFee          = "user/fees"
	cryptocomOrder            = "private/create-order"
	cryptocomCancelOrder      = "private/cancel-order"
	cryptocomCancelAllOrders = "private/cancel-all-orders"
	cryptocomPegOrder         = "order/peg"
	cryptocomPendingOrders    = "private/get-open-orders"
	cryptocomCancelAllAfter   = "order/cancelAllAfter"
	cryptocomWsAuth           = "public/auth"
)

// FetchFundingHistory gets funding history
func (c *Cryptocom) FetchFundingHistory(symbol string) (map[string][]FundingHistoryData, error) {
	var resp map[string][]FundingHistoryData
	params := url.Values{}
	if symbol != "" {
		params.Set("symbol", symbol)
	}
	return resp, c.SendHTTPRequest(exchange.RestFutures, http.MethodGet, cryptocomFuturesFunding+params.Encode(), &resp, false, queryFunc)
}

// GetMarketSummary stores market summary data
func (c *Cryptocom) GetMarketSummary(symbol string, spot bool) ([]Instrument, error) {
	var m InstrumentsResp
	path := cryptocomMarketOverview
	return m.Result.Instruments, c.SendHTTPRequest(exchange.RestSpot, http.MethodGet, path, &m, spot, queryFunc)
}

// FetchOrderBook gets orderbook data for a given pair
func (c *Cryptocom) FetchOrderBook(symbol string, limit int, spot bool) (*Orderbook, error) {
	var o OrderbookResp
	urlValues := url.Values{}
	urlValues.Add("instrument_name", symbol)
	if limit > 0 {
		urlValues.Add("depth", strconv.Itoa(limit))
	}

	err := c.SendHTTPRequest(exchange.RestSpot, http.MethodGet,
		common.EncodeURLValues(cryptocomOrderbook, urlValues), &o, spot, queryFunc)
	//fmt.Printf("UpdateOrderbook PPP: %+v\n", o.Result.Data)

	ob := Orderbook{}
	//fmt.Printf("len(o.Result.Data) PPP: %+v\n", len(o.Result.Data))
	if len(o.Result.Data) > 0 {
		ob = o.Result.Data[0]
	}

	return &ob, err
}

// FetchOrderBookL2 retrieve level 2 orderbook for requested symbol and depth
func (c *Cryptocom) FetchOrderBookL2(symbol string, depth int) (*Orderbook, error) {
	var o Orderbook
	urlValues := url.Values{}
	urlValues.Add("symbol", symbol)
	urlValues.Add("depth", strconv.FormatInt(int64(depth), 10))
	endpoint := common.EncodeURLValues(cryptocomOrderbook+"/L2", urlValues)
	return &o, c.SendHTTPRequest(exchange.RestSpot, http.MethodGet, endpoint, &o, true, queryFunc)
}

// GetTrades returns a list of trades for the specified symbol
func (c *Cryptocom) GetTrades(symbol string, start, end time.Time, beforeSerialID, afterSerialID, count int, includeOld, spot bool) ([]Trade, error) {
	var t []Trade
	urlValues := url.Values{}
	urlValues.Add("symbol", symbol)
	if count > 0 {
		urlValues.Add("count", strconv.Itoa(count))
	}
	if !start.IsZero() {
		urlValues.Add("start", strconv.FormatInt(start.Unix(), 10))
	}
	if !end.IsZero() {
		urlValues.Add("end", strconv.FormatInt(end.Unix(), 10))
	}
	if !start.IsZero() && !end.IsZero() && start.After(end) {
		return t, errors.New("start cannot be after end time")
	}
	if beforeSerialID > 0 {
		urlValues.Add("beforeSerialId", strconv.Itoa(beforeSerialID))
	}
	if afterSerialID > 0 {
		urlValues.Add("afterSerialId", strconv.Itoa(afterSerialID))
	}
	if includeOld {
		urlValues.Add("includeOld", "true")
	}
	return t, c.SendHTTPRequest(exchange.RestSpot, http.MethodGet,
		common.EncodeURLValues(cryptocomTrades, urlValues), &t, spot, queryFunc)
}

// OHLCV retrieve and return OHLCV candle data for requested symbol
func (c *Cryptocom) OHLCV(symbol string, start, end time.Time, resolution int) (OHLCV, error) {
	var o OHLCV
	urlValues := url.Values{}
	urlValues.Add("symbol", symbol)

	if !start.IsZero() && !end.IsZero() {
		if start.After(end) {
			return o, errors.New("start cannot be after end time")
		}
		urlValues.Add("start", strconv.FormatInt(start.Unix(), 10))
		urlValues.Add("end", strconv.FormatInt(end.Unix(), 10))
	}
	var res = 60
	if resolution != 0 {
		res = resolution
	}
	urlValues.Add("resolution", strconv.FormatInt(int64(res), 10))
	endpoint := common.EncodeURLValues(cryptocomOHLCV, urlValues)
	return o, c.SendHTTPRequest(exchange.RestSpot, http.MethodGet, endpoint, &o, true, queryFunc)
}

// GetTickers get all tickers
func (c *Cryptocom) GetTickers(symbol string) (Tickers, error) {
	var p TickersResp
	path := cryptocomPrice
	if symbol != "" {
		path += "?instrument_name=" + url.QueryEscape(symbol)
	}
	return p.Result, c.SendHTTPRequest(exchange.RestSpot, http.MethodGet, path, &p, true, queryFunc)
}

// GetPrice get current price for requested symbol
//func (c *Cryptocom) GetPrice(symbol string) (Price, error) {
//	var p Price
//	path := cryptocomPrice + "?symbol=" + url.QueryEscape(symbol)
//	return p, c.SendHTTPRequest(exchange.RestSpot, http.MethodGet, path, &p, true, queryFunc)
//}

// GetServerTime returns the exchanges server time
func (c *Cryptocom) GetServerTime() (*ServerTime, error) {
	var s ServerTime
	return &s, c.SendHTTPRequest(exchange.RestSpot, http.MethodGet, cryptocomTime, &s, true, queryFunc)
}

// GetWalletInformation returns the users account balance
func (c *Cryptocom) GetWalletInformation() ([]Bals, error) {
	var a GetAccountSummary
	return a.Result.Accounts, c.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost, cryptocomWallet, true, nil, nil, &a, queryFunc)
}

// GetFeeInformation retrieve fee's (maker/taker) for requested symbol
func (c *Cryptocom) GetFeeInformation(symbol string) ([]AccountFees, error) {
	var resp []AccountFees
	urlValues := url.Values{}
	if symbol != "" {
		urlValues.Add("symbol", symbol)
	}
	return resp, c.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodGet, cryptocomUserFee, true, urlValues, nil, &resp, queryFunc)
}

// GetWalletHistory returns the users account balance
func (c *Cryptocom) GetWalletHistory(symbol string, start, end time.Time, count int) (WalletHistory, error) {
	var resp WalletHistory

	urlValues := url.Values{}
	if symbol != "" {
		urlValues.Add("symbol", symbol)
	}
	if !start.IsZero() && !end.IsZero() {
		if start.After(end) || end.Before(start) {
			return resp, errors.New("start cannot be after end time")
		}
		urlValues.Add("start", strconv.FormatInt(start.Unix(), 10))
		urlValues.Add("end", strconv.FormatInt(end.Unix(), 10))
	}
	if count > 0 {
		urlValues.Add("count", strconv.Itoa(count))
	}
	return resp, c.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodGet, cryptocomWalletHistory, true, urlValues, nil, &resp, queryFunc)
}

// WalletWithdrawal submit request to withdraw crypto currency
func (c *Cryptocom) WalletWithdrawal(currency, address, tag, amount string) (WithdrawalResponse, error) {
	var resp WithdrawalResponse
	req := make(map[string]interface{}, 4)
	req["currency"] = currency
	req["address"] = address
	req["tag"] = tag
	req["amount"] = amount
	return resp, c.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost, cryptocomWalletWithdrawal, true, nil, req, &resp, queryFunc)
}

// CreateOrder creates an order
func (c *Cryptocom) CreateOrder(clOrderID string, deviation float64, postOnly bool, price float64, side string, size, stealth, stopPrice float64, symbol, timeInForce string, trailValue, triggerPrice float64, txType, orderType string) (CreateOrder, error) {
	req := make(map[string]interface{})
	if clOrderID != "" {
		req["client_oid"] = clOrderID
	}
	if deviation > 0.0 {
		req["notional"] = deviation
	}
	//if postOnly {
	//	req["postOnly"] = postOnly
	//}
	if price > 0.0 {
		req["price"] = price
	}
	if side != "" {
		req["side"] = side
	}
	if size > 0.0 {
		req["quantity"] = size
	}
	if symbol != "" {
		req["instrument_name"] = symbol
	}
	if timeInForce != "" {
		//req["time_in_force"] = timeInForce
	}
	if triggerPrice > 0.0 {
		req["trigger_price"] = triggerPrice
	}
	if orderType != "" {
		req["type"] = orderType
	}

	var r CreateOrderResp
	return r.Result, c.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost, cryptocomOrder, true, url.Values{}, req, &r, orderFunc)
}

// GetOrders returns all pending orders
func (c *Cryptocom) GetOrders(symbol, orderID, clOrderID string) ([]OrderActive, error) {
	//req := url.Values{}
	//if orderID != "" {
	//	req.Add("orderID", orderID)
	//}
	//
	//
	//if clOrderID != "" {
	//	req.Add("clOrderID", clOrderID)
	//}
	req := make(map[string]interface{})

	if symbol != "" {
		req["instrument_name"] = symbol
		//req.Add("instrument_name", symbol)
	}
	var o GetOpenOrders
	return o.Result.OrderList, c.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost, cryptocomPendingOrders, true, nil, req, &o, orderFunc)
}

// CancelExistingOrder cancels an order
func (c *Cryptocom) CancelExistingOrder(orderID, symbol string) (CancelOrder, error) {
	var co CancelOrder
	//req := url.Values{}
	req := make(map[string]interface{})
	if orderID == "" {
		return nil, fmt.Errorf("orderID missed")
	}

	if symbol == "" {
		return nil, fmt.Errorf("symbol missed")
	}

	req["order_id"] = orderID
	req["instrument_name"] = symbol

	return co, c.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost, cryptocomCancelOrder, true, nil, req, &c, orderFunc)
}

// CancelExistingOrder cancels an order
func (c *Cryptocom) CancelAllExistingOrders(symbol string) (CancelOrder, error) {
	var co CancelOrder
	req := make(map[string]interface{})
	if symbol == "" {
		return nil, fmt.Errorf("symbol missed")
	}

	req["instrument_name"] = symbol

	return co, c.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost, cryptocomCancelAllOrders, true, nil, req, &c, orderFunc)
}

// TradeHistory returns previous trades on exchange
func (c *Cryptocom) TradeHistory(symbol string, start, end time.Time, beforeSerialID, afterSerialID, count int, includeOld bool, clOrderID, orderID string) (TradeHistory, error) {
	var resp TradeHistory
	urlValues := url.Values{}
	if symbol != "" {
		urlValues.Add("symbol", symbol)
	}
	if !start.IsZero() && !end.IsZero() {
		if start.After(end) || end.Before(start) {
			return resp, errors.New("start and end must both be valid")
		}
		urlValues.Add("start", strconv.FormatInt(start.Unix(), 10))
		urlValues.Add("end", strconv.FormatInt(end.Unix(), 10))
	}
	if beforeSerialID > 0 {
		urlValues.Add("beforeSerialId", strconv.Itoa(beforeSerialID))
	}
	if afterSerialID > 0 {
		urlValues.Add("afterSerialId", strconv.Itoa(afterSerialID))
	}
	if includeOld {
		urlValues.Add("includeOld", "true")
	}
	if count > 0 {
		urlValues.Add("count", strconv.Itoa(count))
	}
	if clOrderID != "" {
		urlValues.Add("clOrderId", clOrderID)
	}
	if orderID != "" {
		urlValues.Add("orderID", orderID)
	}
	return resp, c.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodGet, cryptocomExchangeHistory, true, urlValues, nil, &resp, queryFunc)
}

// SendHTTPRequest sends an HTTP request to the desired endpoint
func (c *Cryptocom) SendHTTPRequest(ep exchange.URL, method, endpoint string, result interface{}, spotEndpoint bool, f request.EndpointLimit) error {
	ePoint, err := c.API.Endpoints.GetURL(ep)
	if err != nil {
		return err
	}
	p := cryptocomSPOTAPIPath
	if !spotEndpoint {
		p =cryptocomFuturesAPIPath
	}
	return c.SendPayload(context.Background(), &request.Item{
		Method:        method,
		Path:          ePoint + p + endpoint,
		Result:        result,
		Verbose:       c.Verbose,
		HTTPDebugging: c.HTTPDebugging,
		HTTPRecording: c.HTTPRecording,
		Endpoint:      f,
	})
}

// SendAuthenticatedHTTPRequest sends an authenticated HTTP request to the desired endpoint
func (c *Cryptocom) SendAuthenticatedHTTPRequest(ep exchange.URL, method, endp string, isSpot bool, values url.Values, req map[string]interface{}, result interface{}, f request.EndpointLimit) error {
	if !c.AllowAuthenticatedRequest() {
		return fmt.Errorf(exchange.WarningAuthenticatedRequestWithoutCredentialsSet,
			c.Name)
	}

	ePoint, err := c.API.Endpoints.GetURL(ep)
	if err != nil {
		return err
	}

	//fmt.Println("ePoint:", ePoint)
	//fmt.Println("method:", method)
	//fmt.Println("endpoint:", endp)
	//fmt.Println("isSpot:", isSpot)
	//fmt.Println("values:", values)
	//fmt.Println("req:", req)
	//fmt.Println("Key:", c.API.Credentials.Key)
	//fmt.Println("Sec:", c.API.Credentials.Secret)

	if req == nil {
		req = make(map[string]interface{})
	}

	var paramsString string
	keys := make([]string, 0, len(req))
	for k := range req {
		keys = append(keys, k)
	}
	sort.Strings(keys) // sort params map by keys

	for i := range keys {
		var value string
		switch req[keys[i]].(type) { // to string depending on value type
		case int:
			value = strconv.Itoa(req[keys[i]].(int))
		case int64:
			value = strconv.FormatInt(req[keys[i]].(int64), 10)
		case float64:
			value = fmt.Sprint(req[keys[i]].(float64))
		case string:
			value = req[keys[i]].(string)
		}

		paramsString += keys[i] + value
	}

	var endpoint string
	host := ePoint
	if isSpot {
		host += cryptocomSPOTAPIPath + endp
		endpoint = cryptocomSPOTAPIPath + endp
	} else {
		host += cryptocomFuturesAPIPath
		endpoint += cryptocomFuturesAPIPath
	}

	//fmt.Println("host:", host)
	//fmt.Println("endpoint:", endpoint)

	var body io.Reader
	nonce := time.Now().UnixNano()/int64(time.Millisecond)

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	var id int64 = 0

	hmac := crypto.GetHMAC(
		crypto.HashSHA256,
		[]byte(endp + strconv.Itoa(int(id))+ c.API.Credentials.Key + paramsString + fmt.Sprint(nonce)),
		[]byte(c.API.Credentials.Secret),
	)

	r := wsSub{
		ID: id,
		Nonce: nonce,
		Params: req,
		Method: endp,
		ApiKey: c.API.Credentials.Key,
		Sig: crypto.HexEncodeToString(hmac),
	}

	//fmt.Println("nonce", nonce)
	//fmt.Println("auth:", endp + strconv.Itoa(int(id))+ c.API.Credentials.Key + paramsString + fmt.Sprint(nonce))

	reqPayload, err := json.Marshal(r)
	if err != nil {
		return err
	}
	body = bytes.NewBuffer(reqPayload)


	//fmt.Println("reqPayload:", string(reqPayload))

	//if req != nil {
	//
	//} else {
	//	hmac = crypto.GetHMAC(
	//		crypto.HashSHA256,
	//		[]byte((endpoint + fmt.Sprint(nonce))),
	//		[]byte(c.API.Credentials.Secret),
	//	)
	//	if len(values) > 0 {
	//		host += "?" + values.Encode()
	//	}
	//}
	//fmt.Println("hmac", hmac)
	//headers["sign"] = crypto.HexEncodeToString(hmac)

	if c.Verbose {
		log.Debugf(log.ExchangeSys,
			"%s Sending %s request to URL %s",
			c.Name, method, endpoint)
	}

	return c.SendPayload(context.Background(), &request.Item{
		Method:        method,
		Path:          host,
		Headers:       headers,
		Body:          body,
		Result:        result,
		AuthRequest:   true,
		Verbose:       c.Verbose,
		HTTPDebugging: c.HTTPDebugging,
		HTTPRecording: c.HTTPRecording,
		Endpoint:      f,
	})
}

// GetFee returns an estimate of fee based on type of transaction
func (c *Cryptocom) GetFee(feeBuilder *exchange.FeeBuilder) (float64, error) {
	var fee float64

	switch feeBuilder.FeeType {
	case exchange.CryptocurrencyTradeFee:
		fee = c.calculateTradingFee(feeBuilder) * feeBuilder.Amount * feeBuilder.PurchasePrice
	case exchange.CryptocurrencyWithdrawalFee:
		switch feeBuilder.Pair.Base {
		case currency.USDT:
			fee = 1.08
		case currency.TUSD:
			fee = 1.09
		case currency.BTC:
			fee = 0.0005
		case currency.ETH:
			fee = 0.01
		case currency.LTC:
			fee = 0.001
		}
	case exchange.InternationalBankDepositFee:
		fee = getInternationalBankDepositFee(feeBuilder.Amount)
	case exchange.InternationalBankWithdrawalFee:
		fee = getInternationalBankWithdrawalFee(feeBuilder.Amount)
	case exchange.OfflineTradeFee:
		fee = getOfflineTradeFee(feeBuilder.PurchasePrice, feeBuilder.Amount)
	}
	return fee, nil
}

// getOfflineTradeFee calculates the worst case-scenario trading fee
func getOfflineTradeFee(price, amount float64) float64 {
	return 0.001 * price * amount
}

// getInternationalBankDepositFee returns international deposit fee
// Only when the initial deposit amount is less than $1000 or equivalent,
// Cryptocom will charge a small fee (0.25% or $3 USD equivalent, whichever is greater).
// The small deposit fee is charged in whatever currency it comes in.
func getInternationalBankDepositFee(amount float64) float64 {
	var fee float64
	if amount <= 100 {
		fee = amount * 0.0025
		if fee < 3 {
			return 3
		}
	}
	return fee
}

// getInternationalBankWithdrawalFee returns international withdrawal fee
// 0.1% (min25 USD)
func getInternationalBankWithdrawalFee(amount float64) float64 {
	fee := amount * 0.0009

	if fee < 25 {
		return 25
	}
	return fee
}

// calculateTradingFee return fee based on users current fee tier or default values
func (c *Cryptocom) calculateTradingFee(feeBuilder *exchange.FeeBuilder) float64 {
	formattedPair, err := c.FormatExchangeCurrency(feeBuilder.Pair, asset.Spot)
	if err != nil {
		if feeBuilder.IsMaker {
			return 0.001
		}
		return 0.002
	}
	feeTiers, err := c.GetFeeInformation(formattedPair.String())
	if err != nil {
		if feeBuilder.IsMaker {
			return 0.001
		}
		return 0.002
	}
	if feeBuilder.IsMaker {
		return feeTiers[0].MakerFee
	}
	return feeTiers[0].TakerFee
}

func parseOrderTime(timeStr string) (time.Time, error) {
	return time.Parse(common.SimpleTimeFormat, timeStr)
}
