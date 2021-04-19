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
	cryptocomAPIURL         = "https://api.crypto.com" // "https://uat-api.3ona.co" //"https://api.crypto.com"
	cryptocomSPOTAPIPath    = "/v2/"
	cryptocomFuturesAPIPath = "/v2/"

	// Public endpoints
	cryptocomMarketOverview = "public/get-instruments"
	cryptocomOrderbook      = "public/get-book"
	cryptocomTrades         = "public/get-trades"
	cryptocomOHLCV          = "ohlcv"
	cryptocomPrice          = "public/get-ticker"
	cryptocomFuturesFunding = "funding_history"

	// Authenticated endpoints
	cryptocomWallet           = "private/get-account-summary"
	cryptocomWalletHistory    = "private/get-withdrawal-history"
	cryptocomWalletWithdrawal = "private/create-withdrawal"
	cryptocomExchangeHistory  = "private/get-order-history"
	cryptocomUserFee          = "user/fees"
	cryptocomOrder            = "private/create-order"
	cryptocomCancelOrder      = "private/cancel-order"
	cryptocomCancelAllOrders = "private/cancel-all-orders"
	cryptocomPendingOrders    = "private/get-open-orders"
	cryptocomOrderDetail      ="private/get-order-detail"
	cryptocomWsAuth           = "public/auth"

	// Reason Codes

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
func (c *Cryptocom) CreateOrder(clOrderID string, deviation float64, postOnly bool, price float64, side string, size float64, symbol, timeInForce string, triggerPrice float64, orderType string) (vvv CreateOrder, err error) {
	//if !c.GetAuthenticatedAPISupport(exchange.WebsocketAuthentication) { // if no websocket
	//	return vvv, fmt.Errorf("websocket disabled")
	//}

	if !c.IsWebsocketEnabled() {
		return vvv, fmt.Errorf("websocket disabled")
	}

	if c.API.AuthenticatedWebsocketSupport {
		// return vvv, fmt.Errorf("websocket Auth disabled")
	}

	if !c.Websocket.IsConnected() || c.Websocket.IsConnecting() {
		return vvv, fmt.Errorf("websocket not connected")
	}

	req := make(map[string]interface{})
	if symbol != "" {
		req["instrument_name"] = symbol
	}
	if side != "" {
		req["side"] = side
	}
	if orderType != "" {
		req["type"] = orderType
	}
	if price > 0.0 {
		req["price"] = price
	}

	if orderType == "MARKET" && side == "BUY" {
		req["notional"] = size
	}else{
		if size > 0.0 {
			req["quantity"] = size
		}
	}

	if clOrderID != "" {
		req["client_oid"] = clOrderID
	}
	if timeInForce != "" {
		req["time_in_force"] = timeInForce
	}
	if postOnly {
		req["exec_inst"] = postOnly
	}
	if triggerPrice > 0.0 {
		req["trigger_price"] = triggerPrice
	}

	var r CreateOrderResp
	fmt.Printf("Cryptocom NEW ORDER: %+v\n", req)
	err = c.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost, cryptocomOrder, true, url.Values{}, req, &r, orderFunc)
	if err != nil {
		return vvv, err
	}

	vvv = r.Result

	m, err := LocalMatcher.set(r.Result.OrderID)
	if err != nil {
		return vvv,fmt.Errorf("matcher set fail, unknown state")
	}
	defer m.Cleanup()

	fmt.Printf("MATCHER SET: %+v\n", m.sig) // 1365258593124093057

	timer := time.NewTimer(time.Second * 6)
	select {
	case payload := <-m.C:
		fmt.Println("payload read:", payload)
		switch payload.Status {
		case "REJECTED":
			fmt.Println("REJECTED reason:", payload.Reason)
			return vvv, fmt.Errorf("Insufficient funds")
		case "ACTIVE":
			fmt.Println("ACTIVE")
			vvv.OrderPlaced = true
		case "FILLED":
			fmt.Println("FILLED")
			vvv.OrderPlaced = true
		case "CANCELED":
			fmt.Println("CANCELED")
			//vvv.OrderPlaced = true
		default:
			return vvv, fmt.Errorf("default, unknown state for order_id:%s", r.Result.OrderID)
		}
	case <-timer.C:
		fmt.Println("STOP timer for", r.Result.OrderID)
		timer.Stop()
		return vvv, fmt.Errorf("timeout, unknown state for order_id:%s", r.Result.OrderID)
	}

	return vvv, nil
}

// GetOrders returns all pending orders
func (c *Cryptocom) GetOrders(symbol, orderID, clOrderID string) ([]Order, error) {
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

// GetOrderDetail returns info for specified order_id
func (c *Cryptocom) GetOrderDetail(orderID string) (DetailResult, error) {
	if orderID == "" {
		return DetailResult{}, fmt.Errorf("no orderID passed")
	}

	req := make(map[string]interface{})
	req["order_id"] = orderID
	var o OrderDetail
	return o.Result, c.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost, cryptocomOrderDetail, true, nil, req, &o, orderFunc)
}

// CancelExistingOrder cancels an order
func (c *Cryptocom) CancelExistingOrder(orderID, symbol string) (CancelOrder, error) {
	var co CancelOrder
	//req := url.Values{}
	req := make(map[string]interface{})
	if orderID == "" {
		return co, fmt.Errorf("orderID missed")
	}

	if symbol == "" {
		return co, fmt.Errorf("symbol missed")
	}

	req["order_id"] = orderID
	req["instrument_name"] = symbol
	err :=  c.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost, cryptocomCancelOrder, true, nil, req, &co, orderFunc)
	if err != nil {
		return co, err
	}

	if !c.GetAuthenticatedAPISupport(exchange.WebsocketAuthentication) { // if no websocket
		return co, fmt.Errorf("status is: unknown state" )
	}

	m, err := LocalMatcher.set(orderID)
	if err != nil {
		return co, fmt.Errorf("matcher set fail, unknown state")
	}
	defer m.Cleanup()

	fmt.Printf("CancelExistingOrder MATCHER SET: %+v\n", m)

	timer := time.NewTimer(time.Second * 3)
	select {
	case payload := <-m.C:
		fmt.Println("CancelExistingOrder payload read:", payload)
		switch payload.Status {
		case "CANCELED":
			fmt.Println(orderID, "CANCELED")
			co.Success = true
			return co, nil
		default:
			return co, fmt.Errorf("CancelExistingOrder, status is: %s", payload.Status )
		}
	case <-timer.C:
		fmt.Println("CancelExistingOrder STOP timer")
		timer.Stop()
		return co, fmt.Errorf("no active order found")
	}
}

// CancelAllOrdersByMarket cancels an order by pair
func (c *Cryptocom) CancelAllOrdersByMarket(symbol string) (CancelOrder, error) {
	var co CancelOrder
	req := make(map[string]interface{})
	if symbol == "" {
		return co, fmt.Errorf("symbol missed")
	}

	req["instrument_name"] = symbol
	err := c.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost, cryptocomCancelAllOrders, true, nil, req, &co, orderFunc)
	if err != nil {
		return co, err
	}

	if !c.GetAuthenticatedAPISupport(exchange.WebsocketAuthentication) { // if no websocket
		return co, fmt.Errorf("status is: unknown state" )
	}

	signature := time.Now().UnixNano()
	m, err := LocalMatcher.set(signature)
	if err != nil {
		return co, fmt.Errorf("matcher set fail, unknown state")
	}
	defer m.Cleanup()

	timer := time.NewTimer(time.Second * 3)
	select {
	case payload := <-m.C:
		fmt.Println("CancelAllOrdersByMarket payload read:", payload)
		switch payload.Status {
		case "CANCELED":
			fmt.Println( "CancelAllOrdersByMarket CANCELED")
			co.Success = true
			return co, nil
		default:
			return co, fmt.Errorf("CancelAllOrdersByMarket, status is: %s", payload.Status )
		}
	case <-timer.C:
		fmt.Println("CancelAllOrdersByMarket STOP timer")
		timer.Stop()
		return co, fmt.Errorf("no active order found")
	}
}

// OrderHistory returns list of all orders
func (c *Cryptocom) OrderHistory(symbol string, start, end time.Time, page, pageSize int) ([]Order, error) {
	req := make(map[string]interface{})
	urlValues := url.Values{}

	if !start.IsZero() && !end.IsZero() {
		if start.After(end) || end.Before(start) {
			return nil, errors.New("start and end must both be valid")
		}
		req["start_ts"] = strconv.FormatInt(start.Unix(), 10)
		req["end_ts"] = strconv.FormatInt(end.Unix(), 10)
	}
	if page > 0 {
		req["page"] = page
	}
	if pageSize > 0 {
		req["page_size"] = pageSize
	}
	if symbol != "" {
		req["instrument_name"] = symbol
	}

	var response GetOpenOrders
	return response.Result.OrderList, c.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost, cryptocomExchangeHistory, true, urlValues, req, &response, queryFunc)
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

	// fmt.Println("nonce", nonce)
	// fmt.Println("auth:", endp + strconv.Itoa(int(id))+ c.API.Credentials.Key + paramsString + fmt.Sprint(nonce))

	reqPayload, err := json.Marshal(r)
	if err != nil {
		return err
	}
	body = bytes.NewBuffer(reqPayload)

	//fmt.Println("reqPayload:", string(reqPayload))

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
	}
	return fee, nil
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
