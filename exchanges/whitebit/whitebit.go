package whitebit

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/thrasher-corp/gocryptotrader/common"
	"github.com/thrasher-corp/gocryptotrader/common/crypto"
	"github.com/thrasher-corp/gocryptotrader/currency"
	exchange "github.com/thrasher-corp/gocryptotrader/exchanges"
	"github.com/thrasher-corp/gocryptotrader/exchanges/order"
	"github.com/thrasher-corp/gocryptotrader/exchanges/request"
	"github.com/thrasher-corp/gocryptotrader/log"
	"github.com/thrasher-corp/gocryptotrader/portfolio/withdraw"
)

const (
	whitebitWSAPIURLBase = "wss://api.whitebit.com/ws"
	whitebitAPIURLBase = "https://whitebit.com"
	// Version 1 API endpoints
	whitebitAPIVersion         = "/v4/"
	whitebitStats              = "api/v4/public/time"
	whitebitAccountInfo        = "account_infos"
	whitebitAccountFees        = "account_fees"
	whitebitAccountSummary     = "summary"
	whitebitDeposit            = "deposit/new"
	whitebitBalances           = "trade-account/balance"
	whitebitTransfer           = "transfer"
	whitebitWithdrawal         = "withdraw"
	whitebitMarketOrderNew     = "order/market"
	whitebitLimitOrderNew      = "order/new"
	whitebitOrderCancel        = "order/cancel"
	whitebitOrderCancelMulti   = "order/cancel/multi"
	whitebitOrderCancelAll     = "order/cancel/all"
	whitebitOrderCancelReplace = "order/cancel/replace"
	whitebitOrderStatus        = "trade-account/order"
	whitebitInactiveOrders     = "orders/hist"
	whitebitOrders             = "orders"
	whitebitPositions          = "positions"
	whitebitHistory            = "history"
	whitebitHistoryMovements   = "history/movements"
	whitebitTradeHistory       = "mytrades"

	// Version 2 API endpoints
	whitebitAPIVersion2     = "/api/v4/"
	whitebitV2Balances      = "trade-account/balance"
	whitebitV2AccountInfo   = "auth/r/info/user"
	whitebitV2FundingInfo   = "auth/r/info/funding/%s"
	whitebitPlatformStatus  = "platform/status"
	whitebitTickerBatch     = "public/ticker"
	whitebitTicker          = "ticker/"
	whitebitTrades          = "trades/"
	whitebitOrderbook       = "public/orderbook/"
	whitebitCandles         = "candles/trade"
	whitebitKeyPermissions  = "key_info"
	whitebitDepositMethod   = "main-account/address"

	bitfinexMaintenanceMode = 0
	bitfinexOperativeMode   = 1

	witebitGetWsToken = "profile/websocket_token"
)

// Bitfinex is the overarching type across the bitfinex package
type Whitebit struct {
	exchange.Base
	WebsocketSubdChannels map[int]WebsocketChanInfo
}

// GetPlatformStatus returns the Bifinex platform status
func (b *Whitebit) GetPlatformStatus() (int, error) {
	var response []int
	err := b.SendHTTPRequest(exchange.RestSpot,
		whitebitAPIVersion2+
			whitebitPlatformStatus,
		&response,
		platformStatus)
	if err != nil {
		return -1, err
	}

	switch response[0] {
	case bitfinexOperativeMode:
		return bitfinexOperativeMode, nil
	case bitfinexMaintenanceMode:
		return bitfinexMaintenanceMode, nil
	}

	return -1, fmt.Errorf("unexpected platform status value %d", response[0])
}

// GetV2FundingInfo gets funding info for margin pairs
func (b *Whitebit) GetV2FundingInfo(key string) (MarginFundingDataV2, error) {
	var resp []interface{}
	var response MarginFundingDataV2
	err := b.SendAuthenticatedHTTPRequestV2(exchange.RestSpot, http.MethodPost,
		fmt.Sprintf(whitebitV2FundingInfo, key),
		nil,
		&resp,
		getAccountFees)
	if err != nil {
		return response, err
	}
	if len(resp) != 3 {
		return response, errors.New("invalid data received")
	}
	sym, ok := resp[0].(string)
	if !ok {
		return response, errors.New("failed type assertion for sym")
	}
	symbol, ok := resp[1].(string)
	if !ok {
		return response, errors.New("failed type assertion for symbol")
	}
	fundingData, ok := resp[2].([]interface{})
	if !ok {
		return response, errors.New("failed type assertion for fundingData")
	}
	response.Sym = sym
	response.Symbol = symbol
	if len(fundingData) < 4 {
		return response, errors.New("invalid length of fundingData")
	}
	for x := 0; x < 3; x++ {
		_, ok := fundingData[x].(float64)
		if !ok {
			return response, fmt.Errorf("type conversion failed for x = %d", x)
		}
	}
	response.Data.YieldLoan = fundingData[0].(float64)
	response.Data.YieldLend = fundingData[1].(float64)
	response.Data.DurationLoan = fundingData[2].(float64)
	response.Data.DurationLend = fundingData[3].(float64)
	return response, nil
}

// GetAccountInfoV2 gets V2 account data
func (b *Whitebit) GetAccountInfoV2() (AccountV2Data, error) {
	var resp AccountV2Data
	var data []interface{}
	err := b.SendAuthenticatedHTTPRequestV2(exchange.RestSpot, http.MethodPost,
		whitebitV2AccountInfo,
		nil,
		&data,
		getAccountFees)
	if err != nil {
		return resp, err
	}
	if len(data) < 8 {
		return resp, errors.New("invalid length of data")
	}
	var ok bool
	var tempString string
	var tempFloat float64
	if tempFloat, ok = data[0].(float64); !ok {
		return resp, errors.New("type assertion failed for id, check for api updates")
	}
	resp.ID = int64(tempFloat)
	if tempString, ok = data[1].(string); !ok {
		return resp, errors.New("type assertion failed for email, check for api updates")
	}
	resp.Email = tempString
	if tempString, ok = data[2].(string); !ok {
		return resp, errors.New("type assertion failed for username, check for api updates")
	}
	resp.Username = tempString
	if tempFloat, ok = data[3].(float64); !ok {
		return resp, errors.New("type assertion failed for accountcreate, check for api updates")
	}
	resp.MTSAccountCreate = int64(tempFloat)
	if tempFloat, ok = data[4].(float64); !ok {
		return resp, errors.New("type assertion failed for verified, check for api updates")
	}
	resp.Verified = int64(tempFloat)
	if tempString, ok = data[7].(string); !ok {
		return resp, errors.New("type assertion failed for timezone, check for api updates")
	}
	resp.Timezone = tempString
	return resp, nil
}

// GetV2Balances gets v2 balances
func (b *Whitebit) GetV2Balances() ([]WalletDataV2, error) {
	var resp []WalletDataV2
	var data [][4]interface{}
	err := b.SendAuthenticatedHTTPRequestV2(exchange.RestSpot, http.MethodPost,
		whitebitV2Balances,
		nil,
		&data,
		getAccountFees)
	if err != nil {
		return resp, err
	}
	for x := range data {
		wType, ok := data[x][0].(string)
		if !ok {
			return resp, errors.New("type assertion failed for walletType, check for api updates")
		}
		curr, ok := data[x][1].(string)
		if !ok {
			return resp, errors.New("type assertion failed for currency, check for api updates")
		}
		bal, ok := data[x][2].(float64)
		if !ok {
			return resp, errors.New("type assertion failed for balance, check for api updates")
		}
		unsettledInterest, ok := data[x][3].(float64)
		if !ok {
			return resp, errors.New("type assertion failed for unsettledInterest, check for api updates")
		}
		resp = append(resp, WalletDataV2{
			WalletType:        wType,
			Currency:          curr,
			Balance:           bal,
			UnsettledInterest: unsettledInterest,
		})
	}
	return resp, nil
}


// GetTickerBatch returns all supported ticker information
func (b *Whitebit) GetTickerBatch() (map[string]Ticker, error) {
	type Asset struct {
		BaseId int64 `json:"base_id"`
		QuoteId int64 `json:"quote_id"`
		LastPrice float64 `json:"last_price,string"`
		QuoteVolume float64 `quote_volume,string`
		BaseVolume float64 `json:"base_volume,string"`
		IsFrozen   bool `json:"isFrozen"`
		Change float64 `json:"change,string"`
	}

	var response map[string]Asset

	path := whitebitAPIVersion2 + whitebitTickerBatch

	err := b.SendHTTPRequest(exchange.RestSpot, path, &response, tickerBatch)
	if err != nil {
		return nil, err
	}

	var tickers = make(map[string]Ticker)
	for pair, v := range response {
		tickers[pair] = Ticker{
			DailyChange:     v.Change,
			//DailyChangePerc: v.Change, // ensure which one
			Last:            v.LastPrice,
			Volume:          v.BaseVolume,
		}
	}

	return tickers, nil
}

// GetTicker returns ticker informatijsonon for one symbol
func (b *Whitebit) GetTicker(symbol string) (Ticker, error) {
	var response []interface{}

	path := whitebitAPIVersion2 + whitebitTicker + symbol

	err := b.SendHTTPRequest(exchange.RestSpot, path, &response, tickerFunction)
	if err != nil {
		return Ticker{}, err
	}

	if len(response) > 10 {
		return Ticker{
			FlashReturnRate:    response[0].(float64),
			Bid:                response[1].(float64),
			BidPeriod:          int64(response[2].(float64)),
			BidSize:            response[3].(float64),
			Ask:                response[4].(float64),
			AskPeriod:          int64(response[5].(float64)),
			AskSize:            response[6].(float64),
			DailyChange:        response[7].(float64),
			DailyChangePerc:    response[8].(float64),
			Last:               response[9].(float64),
			Volume:             response[10].(float64),
			High:               response[11].(float64),
			Low:                response[12].(float64),
			FFRAmountAvailable: response[15].(float64),
		}, nil
	}
	return Ticker{
		Bid:             response[0].(float64),
		BidSize:         response[1].(float64),
		Ask:             response[2].(float64),
		AskSize:         response[3].(float64),
		DailyChange:     response[4].(float64),
		DailyChangePerc: response[5].(float64),
		Last:            response[6].(float64),
		Volume:          response[7].(float64),
		High:            response[8].(float64),
		Low:             response[9].(float64),
	}, nil
}

// GetTrades gets historic trades that occurred on the exchange
//
// currencyPair e.g. "tBTCUSD"
// timestampStart is a millisecond timestamp
// timestampEnd is a millisecond timestamp
// reOrderResp reorders the returned data.
func (b *Whitebit) GetTrades(currencyPair string, limit, timestampStart, timestampEnd int64, reOrderResp bool) ([]Trade, error) {
	v := url.Values{}
	if limit > 0 {
		v.Set("limit", strconv.FormatInt(limit, 10))
	}

	if timestampStart > 0 {
		v.Set("start", strconv.FormatInt(timestampStart, 10))
	}

	if timestampEnd > 0 {
		v.Set("end", strconv.FormatInt(timestampEnd, 10))
	}
	sortVal := "0"
	if reOrderResp {
		sortVal = "1"
	}
	v.Set("sort", sortVal)

	path := whitebitAPIVersion2 + whitebitTrades + currencyPair + "/hist" + "?" + v.Encode()

	var resp [][]interface{}
	err := b.SendHTTPRequest(exchange.RestSpot, path, &resp, tradeRateLimit)
	if err != nil {
		return nil, err
	}

	var history []Trade
	for i := range resp {
		amount := resp[i][2].(float64)
		side := order.Buy.String()
		if amount < 0 {
			side = order.Sell.String()
			amount *= -1
		}

		if len(resp[i]) > 4 {
			history = append(history, Trade{
				TID:       int64(resp[i][0].(float64)),
				Timestamp: int64(resp[i][1].(float64)),
				Amount:    amount,
				Rate:      resp[i][3].(float64),
				Period:    int64(resp[i][4].(float64)),
				Type:      side,
			})
			continue
		}

		history = append(history, Trade{
			TID:       int64(resp[i][0].(float64)),
			Timestamp: int64(resp[i][1].(float64)),
			Amount:    amount,
			Price:     resp[i][3].(float64),
			Type:      side,
		})
	}

	return history, nil
}

// GetOrderbook retieves the orderbook bid and ask price points for a currency
// pair - By default the response will return 25 bid and 25 ask price points.
// Values can contain limit amounts for both the asks and bids - Example
// "len" = 100
func (b *Whitebit) GetOrderbook(symbol, precision string, limit int64) (Orderbook, error) {
	var u = url.Values{}
	if limit > 0 {
		u.Set("len", strconv.FormatInt(limit, 10))
	}
	path := whitebitAPIVersion2 + whitebitOrderbook + symbol + "?depth=" + precision + "&level=2"

	type OrderBook struct {
		Timestamp int64
		Asks [][]string `json:"asks,string"`
		Bids [][]string `json:"bids,string"`
	}

	var response OrderBook
	err := b.SendHTTPRequest(exchange.RestSpot, path, &response, orderbookFunction)
	if err != nil {
		return Orderbook{}, err
	}

	var o Orderbook

	for x := range response.Bids {
		var b Book
		b.Amount = strToFloat(response.Bids[x][1])
		b.Price = strToFloat(response.Bids[x][0])
		b.Rate = strToFloat(response.Bids[x][0])
		o.Bids = append(o.Bids, b)
	}

	for x := range response.Asks {
		var b Book
		b.Amount = strToFloat(response.Asks[x][1])
		b.Price = strToFloat(response.Asks[x][0])
		b.Rate = strToFloat(response.Asks[x][0])
		o.Asks = append(o.Asks, b)
	}

	return o, nil
}

// strToFloat converter
func strToFloat (s string) float64{
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// GetStats returns various statistics about the requested pair
func (b *Whitebit) GetStats(symbol string) ([]Stat, error) {
	var response []Stat
	path := whitebitAPIVersion + whitebitStats + symbol
	return response, b.SendHTTPRequest(exchange.RestSpot, path, &response, statsV1)
}

// GetCandles returns candle chart data
// timeFrame values: '1m', '5m', '15m', '30m', '1h', '3h', '6h', '12h', '1D',
// '7D', '14D', '1M'
// section values: last or hist
func (b *Whitebit) GetCandles(symbol, timeFrame string, start, end int64, limit uint32, historic bool) ([]Candle, error) {
	var fundingPeriod string
	if symbol[0] == 'f' {
		fundingPeriod = ":p30"
	}

	var path = whitebitAPIVersion2 +
		whitebitCandles +
		":" +
		timeFrame +
		":" +
		symbol +
		fundingPeriod

	if historic {
		v := url.Values{}
		if start > 0 {
			v.Set("start", strconv.FormatInt(start, 10))
		}

		if end > 0 {
			v.Set("end", strconv.FormatInt(end, 10))
		}

		if limit > 0 {
			v.Set("limit", strconv.FormatInt(int64(limit), 10))
		}

		path += "/hist"
		if len(v) > 0 {
			path += "?" + v.Encode()
		}

		var response [][]interface{}
		err := b.SendHTTPRequest(exchange.RestSpot, path, &response, candle)
		if err != nil {
			return nil, err
		}

		var c []Candle
		for i := range response {
			c = append(c, Candle{
				Timestamp: time.Unix(int64(response[i][0].(float64)/1000), 0),
				Open:      response[i][1].(float64),
				Close:     response[i][2].(float64),
				High:      response[i][3].(float64),
				Low:       response[i][4].(float64),
				Volume:    response[i][5].(float64),
			})
		}

		return c, nil
	}

	path += "/last"

	var response []interface{}
	err := b.SendHTTPRequest(exchange.RestSpot, path, &response, candle)
	if err != nil {
		return nil, err
	}

	if len(response) == 0 {
		return nil, errors.New("no data returned")
	}

	return []Candle{{
		Timestamp: time.Unix(int64(response[0].(float64))/1000, 0),
		Open:      response[1].(float64),
		Close:     response[2].(float64),
		High:      response[3].(float64),
		Low:       response[4].(float64),
		Volume:    response[5].(float64),
	}}, nil
}

// GetConfigurations fetchs currency and symbol site configuration data.
func (b *Whitebit) GetConfigurations() error {
	return common.ErrNotYetImplemented
}

// GetStatus returns different types of platform information - currently
// supports derivatives pair status only.
func (b *Whitebit) GetStatus() error {
	return common.ErrNotYetImplemented
}

// GetAccountFees returns information about your account trading fees
func (b *Whitebit) GetAccountFees() ([]AccountInfo, error) {
	var responses []AccountInfo
	return responses, b.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost,
		whitebitAccountInfo,
		nil,
		&responses,
		getAccountFees)
}

// GetWithdrawalFees - Gets all fee rates for withdrawals
func (b *Whitebit) GetWithdrawalFees() (AccountFees, error) {
	response := AccountFees{}
	return response, b.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost,
		whitebitAccountFees,
		nil,
		&response,
		getWithdrawalFees)
}

// GetAccountSummary returns a 30-day summary of your trading volume and return
// on margin funding
func (b *Whitebit) GetAccountSummary() (AccountSummary, error) {
	response := AccountSummary{}

	return response, b.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost,
		whitebitAccountSummary,
		nil,
		&response,
		getAccountSummary)
}

// NewDeposit returns a new deposit address
// Method - Example methods accepted: “bitcoin”, “litecoin”, “ethereum”,
// “tethers", "ethereumc", "zcash", "monero", "iota", "bcash"
// WalletName - accepted: “trading”, “exchange”, “deposit”
// renew - Default is 0. If set to 1, will return a new unused deposit address
func (b *Whitebit) NewDeposit(method, walletName string, renew int) (DepositResponse, error) {
	if !common.StringDataCompare(AcceptedWalletNames, walletName) {
		return DepositResponse{},
			fmt.Errorf("walletname: [%s] is not allowed, supported: %s",
				walletName,
				AcceptedWalletNames)
	}

	response := DepositResponse{}
	req := make(map[string]interface{})
	req["method"] = method
	req["wallet_name"] = walletName
	req["renew"] = renew

	return response, b.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost,
		whitebitDeposit,
		req,
		&response,
		newDepositAddress)
}

// GetKeyPermissions checks the permissions of the key being used to generate
// this request.
func (b *Whitebit) GetKeyPermissions() (KeyPermissions, error) {
	response := KeyPermissions{}
	return response, b.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost,
		whitebitKeyPermissions,
		nil,
		&response,
		getAccountFees)
}

// GetAccountBalance returns full wallet balance information
func (b *Whitebit) GetAccountBalance() (map[string]Balance, error) {
	var response map[string]Balance
	var params map[string]interface{}
	params = make(map[string]interface{})
	return response, b.SendAuthenticatedHTTPRequestV2(exchange.RestSpot, http.MethodPost,
		whitebitBalances,
		params,
		&response,
		getAccountBalance)
}

// WalletTransfer move available balances between your wallets
// Amount - Amount to move
// Currency -  example "BTC"
// WalletFrom - example "exchange"
// WalletTo -  example "deposit"
func (b *Whitebit) WalletTransfer(amount float64, currency, walletFrom, walletTo string) (WalletTransfer, error) {
	var response []WalletTransfer
	req := make(map[string]interface{})
	req["amount"] = strconv.FormatFloat(amount, 'f', -1, 64)
	req["currency"] = currency
	req["walletfrom"] = walletFrom
	req["walletto"] = walletTo

	err := b.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost,
		whitebitTransfer,
		req,
		&response,
		walletTransfer)
	if err != nil {
		return WalletTransfer{}, err
	}

	if response[0].Status == "error" {
		return WalletTransfer{}, errors.New(response[0].Message)
	}
	return response[0], nil
}

// WithdrawCryptocurrency requests a withdrawal from one of your wallets.
// For FIAT, use WithdrawFIAT
func (b *Whitebit) WithdrawCryptocurrency(wallet, address, paymentID string, amount float64, c currency.Code) (Withdrawal, error) {
	var response []Withdrawal
	req := make(map[string]interface{})
	req["withdraw_type"] = b.ConvertSymbolToWithdrawalType(c)
	req["walletselected"] = wallet
	req["amount"] = strconv.FormatFloat(amount, 'f', -1, 64)
	req["address"] = address
	if paymentID != "" {
		req["payment_id"] = paymentID
	}

	err := b.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost,
		whitebitWithdrawal,
		req,
		&response,
		withdrawV1)
	if err != nil {
		return Withdrawal{}, err
	}

	if response[0].Status == "error" {
		return Withdrawal{}, errors.New(response[0].Message)
	}

	return response[0], nil
}

// WithdrawFIAT Sends an authenticated request to withdraw FIAT currency
func (b *Whitebit) WithdrawFIAT(withdrawalType, walletType string, withdrawRequest *withdraw.Request) (Withdrawal, error) {
	var response []Withdrawal
	req := make(map[string]interface{})

	req["withdraw_type"] = withdrawalType
	req["walletselected"] = walletType
	req["amount"] = strconv.FormatFloat(withdrawRequest.Amount, 'f', -1, 64)
	req["account_name"] = withdrawRequest.Fiat.Bank.AccountName
	req["account_number"] = withdrawRequest.Fiat.Bank.AccountNumber
	req["bank_name"] = withdrawRequest.Fiat.Bank.BankName
	req["bank_address"] = withdrawRequest.Fiat.Bank.BankAddress
	req["bank_city"] = withdrawRequest.Fiat.Bank.BankPostalCity
	req["bank_country"] = withdrawRequest.Fiat.Bank.BankCountry
	req["expressWire"] = withdrawRequest.Fiat.IsExpressWire
	req["swift"] = withdrawRequest.Fiat.Bank.SWIFTCode
	req["detail_payment"] = withdrawRequest.Description
	req["currency"] = withdrawRequest.Currency
	req["account_address"] = withdrawRequest.Fiat.Bank.BankAddress

	if withdrawRequest.Fiat.RequiresIntermediaryBank {
		req["intermediary_bank_name"] = withdrawRequest.Fiat.IntermediaryBankName
		req["intermediary_bank_address"] = withdrawRequest.Fiat.IntermediaryBankAddress
		req["intermediary_bank_city"] = withdrawRequest.Fiat.IntermediaryBankCity
		req["intermediary_bank_country"] = withdrawRequest.Fiat.IntermediaryBankCountry
		req["intermediary_bank_account"] = strconv.FormatFloat(withdrawRequest.Fiat.IntermediaryBankAccountNumber, 'f', -1, 64)
		req["intermediary_bank_swift"] = withdrawRequest.Fiat.IntermediarySwiftCode
	}

	err := b.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost,
		whitebitWithdrawal,
		req,
		&response,
		withdrawV1)
	if err != nil {
		return Withdrawal{}, err
	}

	if response[0].Status == "error" {
		return Withdrawal{}, errors.New(response[0].Message)
	}

	return response[0], nil
}

// NewOrder submits a new order and returns a order information
func (b *Whitebit) NewOrder(currencyPair, orderType string, amount, price float64, orderSide string, hidden bool) (Order, error) {
	if !common.StringDataCompare(AcceptedOrderType, orderType) {
		return Order{}, fmt.Errorf("order type %s not accepted", orderType)
	}

	response := Order{}
	req := make(map[string]interface{})
	req["market"] = currencyPair
	req["amount"] = strconv.FormatFloat(amount, 'f', -1, 64)
	req["side"] = strings.ToLower(orderSide)
	req["price"] = strconv.FormatFloat(price, 'f', -1, 64)

	var endpoint string
	switch orderType {
	case "market":
		endpoint = whitebitMarketOrderNew
	case "limit":
		endpoint = whitebitLimitOrderNew
	default:
		endpoint = whitebitMarketOrderNew
	}

	return response, b.SendAuthenticatedHTTPRequestV2(exchange.RestSpot, http.MethodPost,
		endpoint,
		req,
		&response,
		orderV1)
}

// CancelExistingOrder cancels a single order by OrderID
func (b *Whitebit) CancelExistingOrder(pair string, orderID int64) (Order, error) {
	response := Order{}
	req := make(map[string]interface{})
	req["market"] = pair
	req["orderId"] = orderID

	return response, b.SendAuthenticatedHTTPRequestV2(exchange.RestSpot, http.MethodPost,
		whitebitOrderCancel,
		req,
		&response,
		orderMulti)
}

// CancelMultipleOrders cancels multiple orders
func (b *Whitebit) CancelMultipleOrders(orderIDs []int64) (string, error) {
	response := GenericResponse{}
	req := make(map[string]interface{})
	req["order_ids"] = orderIDs

	return response.Result, b.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost,
		whitebitOrderCancelMulti,
		req,
		nil,
		orderMulti)
}

// CancelAllExistingOrders cancels all active and open orders
func (b *Whitebit) CancelAllExistingOrders() (string, error) {
	response := GenericResponse{}

	return response.Result, b.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost,
		whitebitOrderCancelAll,
		nil,
		nil,
		orderMulti)
}

// ReplaceOrder replaces an older order with a new order
func (b *Whitebit) ReplaceOrder(orderID int64, symbol string, amount, price float64, buy bool, orderType string, hidden bool) (Order, error) {
	response := Order{}
	req := make(map[string]interface{})
	req["order_id"] = orderID
	req["symbol"] = symbol
	req["amount"] = strconv.FormatFloat(amount, 'f', -1, 64)
	req["price"] = strconv.FormatFloat(price, 'f', -1, 64)
	req["exchange"] = "bitfinex"
	req["type"] = orderType
	req["is_hidden"] = hidden

	if buy {
		req["side"] = order.Buy.Lower()
	} else {
		req["side"] = order.Sell.Lower()
	}

	return response, b.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost,
		whitebitOrderCancelReplace,
		req,
		&response,
		orderMulti)
}

// GetOrderStatus returns order status information
func (b *Whitebit) GetOrderStatus(orderID int64) (ExecutedOrderDeals, error) {
	var orderStatus ExecutedOrderDeals
	req := make(map[string]interface{})
	req["orderId"] = orderID

	return orderStatus, b.SendAuthenticatedHTTPRequestV2(exchange.RestSpot, http.MethodPost,
		whitebitOrderStatus,
		req,
		&orderStatus,
		orderMulti)
}

// GetInactiveOrders returns order status information
func (b *Whitebit) GetInactiveOrders() ([]Order, error) {
	var response []Order
	req := make(map[string]interface{})
	req["limit"] = "100"

	return response, b.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost,
		whitebitInactiveOrders,
		req,
		&response,
		orderMulti)
}

// GetOpenOrders returns all active orders and statuses
func (b *Whitebit) GetOpenOrders(pair string) ([]Order, error) {
	params := make(map[string]interface{})
	params["market"] = pair

	var response []Order
	return response, b.SendAuthenticatedHTTPRequestV2(exchange.RestSpot, http.MethodPost,
		whitebitOrders,
		params,
		&response,
		orderMulti)
}

// GetActivePositions returns an array of active positions
func (b *Whitebit) GetActivePositions() ([]Position, error) {
	var response []Position

	return response, b.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost,
		whitebitPositions,
		nil,
		&response,
		orderMulti)
}

// GetBalanceHistory returns balance history for the account
func (b *Whitebit) GetBalanceHistory(symbol string, timeSince, timeUntil time.Time, limit int, wallet string) ([]BalanceHistory, error) {
	var response []BalanceHistory
	req := make(map[string]interface{})
	req["currency"] = symbol

	if !timeSince.IsZero() {
		req["since"] = timeSince
	}
	if !timeUntil.IsZero() {
		req["until"] = timeUntil
	}
	if limit > 0 {
		req["limit"] = limit
	}
	if len(wallet) > 0 {
		req["wallet"] = wallet
	}

	return response, b.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost,
		whitebitHistory,
		req,
		&response,
		orderMulti)
}

// GetMovementHistory returns an array of past deposits and withdrawals
func (b *Whitebit) GetMovementHistory(symbol, method string, timeSince, timeUntil time.Time, limit int) ([]MovementHistory, error) {
	var response []MovementHistory
	req := make(map[string]interface{})
	req["currency"] = symbol

	if len(method) > 0 {
		req["method"] = method
	}
	if !timeSince.IsZero() {
		req["since"] = timeSince
	}
	if !timeUntil.IsZero() {
		req["until"] = timeUntil
	}
	if limit > 0 {
		req["limit"] = limit
	}

	return response, b.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost,
		whitebitHistoryMovements,
		req,
		&response,
		orderMulti)
}

// GetTradeHistory returns past executed trades
func (b *Whitebit) GetTradeHistory(currencyPair string, timestamp, until time.Time, limit, reverse int) ([]TradeHistory, error) {
	var response []TradeHistory
	req := make(map[string]interface{})
	req["currency"] = currencyPair
	req["timestamp"] = timestamp

	if !until.IsZero() {
		req["until"] = until
	}
	if limit > 0 {
		req["limit"] = limit
	}
	if reverse > 0 {
		req["reverse"] = reverse
	}

	return response, b.SendAuthenticatedHTTPRequest(exchange.RestSpot, http.MethodPost,
		whitebitTradeHistory,
		req,
		&response,
		orderMulti)
}

// SendHTTPRequest sends an unauthenticated request
func (b *Whitebit) SendHTTPRequest(ep exchange.URL, path string, result interface{}, e request.EndpointLimit) error {
	endpoint, err := b.API.Endpoints.GetURL(ep)
	if err != nil {
		return err
	}
	return b.SendPayload(context.Background(), &request.Item{
		Method:        http.MethodGet,
		Path:          endpoint + path,
		Result:        result,
		Verbose:       b.Verbose,
		HTTPDebugging: b.HTTPDebugging,
		HTTPRecording: b.HTTPRecording,
		Endpoint:      e})
}

// SendAuthenticatedHTTPRequest sends an autheticated http request and json
// unmarshals result to a supplied variable
func (b *Whitebit) SendAuthenticatedHTTPRequest(ep exchange.URL, method, path string, params map[string]interface{}, result interface{}, endpoint request.EndpointLimit) error {
	if !b.AllowAuthenticatedRequest() {
		return fmt.Errorf(exchange.WarningAuthenticatedRequestWithoutCredentialsSet,
			b.Name)
	}

	ePoint, err := b.API.Endpoints.GetURL(ep)
	if err != nil {
		return err
	}

	n := b.Requester.GetNonce(false)

	req := make(map[string]interface{})
	req["request"] = whitebitAPIVersion + path
	req["nonce"] = n.String()

	for key, value := range params {
		req[key] = value
	}

	PayloadJSON, err := json.Marshal(req)
	if err != nil {
		return errors.New("sendAuthenticatedAPIRequest: unable to JSON request")
	}

	if b.Verbose {
		log.Debugf(log.ExchangeSys, "Request JSON: %s\n", PayloadJSON)
	}

	PayloadBase64 := crypto.Base64Encode(PayloadJSON)
	hmac := crypto.GetHMAC(crypto.HashSHA512_384, []byte(PayloadBase64),
		[]byte(b.API.Credentials.Secret))
	headers := make(map[string]string)
	headers["X-BFX-APIKEY"] = b.API.Credentials.Key
	headers["X-BFX-PAYLOAD"] = PayloadBase64
	headers["X-BFX-SIGNATURE"] = crypto.HexEncodeToString(hmac)

	return b.SendPayload(context.Background(), &request.Item{
		Method:        method,
		Path:          ePoint + whitebitAPIVersion2 + path,
		Headers:       headers,
		Result:        result,
		AuthRequest:   true,
		NonceEnabled:  true,
		Verbose:       b.Verbose,
		HTTPDebugging: b.HTTPDebugging,
		HTTPRecording: b.HTTPRecording,
		Endpoint:      endpoint})
}

// SendAuthenticatedHTTPRequestV2 sends an autheticated http request and json
// unmarshals result to a supplied variable
func (b *Whitebit) SendAuthenticatedHTTPRequestV2(ep exchange.URL, method, path string, params map[string]interface{}, result interface{}, endpoint request.EndpointLimit) error {
	if !b.AllowAuthenticatedRequest() {
		return fmt.Errorf(exchange.WarningAuthenticatedRequestWithoutCredentialsSet,
			b.Name)
	}

	ePoint, err := b.API.Endpoints.GetURL(ep)
	if err != nil {
		return err
	}

	if len(params) == 0 {
		params = make(map[string]interface{})
	}

	params["request"] = whitebitAPIVersion2 + path
	params["nonce"] = b.Requester.GetNonce(false).String()

	var payload []byte
	var payload64 string
	if len(params) != 0 {
		payload, err = json.Marshal(params)
		if err != nil {
			return err
		}

		payload64 = base64.StdEncoding.EncodeToString(payload)
	}

	//calculating signature using sha512
	h := hmac.New(sha512.New, []byte(b.API.Credentials.Secret))
	h.Write([]byte(payload64))
	signature := fmt.Sprintf("%x", h.Sum(nil))

	headers := make(map[string]string)
	headers["Content-Type"] = "application/json"
	headers["X-TXC-APIKEY"] = b.API.Credentials.Key
	headers["X-TXC-SIGNATURE"] = signature
	headers["X-TXC-PAYLOAD"] = payload64

	return b.SendPayload(context.Background(), &request.Item{
		Method:        method,
		Path:          ePoint + whitebitAPIVersion2 + path,
		Headers:       headers,
		Body:          bytes.NewBuffer(payload),
		Result:        result,
		AuthRequest:   true,
		NonceEnabled:  true,
		Verbose:       b.Verbose,
		HTTPDebugging: b.HTTPDebugging,
		HTTPRecording: b.HTTPRecording,
		Endpoint:      endpoint,
	})
}

// GetFee returns an estimate of fee based on type of transaction
func (b *Whitebit) GetFee(feeBuilder *exchange.FeeBuilder) (float64, error) {
	var fee float64

	switch feeBuilder.FeeType {
	case exchange.CryptocurrencyTradeFee:
		accountInfos, err := b.GetAccountFees()
		if err != nil {
			return 0, err
		}
		fee, err = b.CalculateTradingFee(accountInfos,
			feeBuilder.PurchasePrice,
			feeBuilder.Amount,
			feeBuilder.Pair.Base,
			feeBuilder.IsMaker)
		if err != nil {
			return 0, err
		}
	case exchange.CyptocurrencyDepositFee:
		//TODO: fee is charged when < $1000USD is transferred, need to infer value in some way
		fee = 0
	case exchange.CryptocurrencyWithdrawalFee:
		acc, err := b.GetWithdrawalFees()
		if err != nil {
			return 0, err
		}
		fee, err = b.GetCryptocurrencyWithdrawalFee(feeBuilder.Pair.Base, acc)
		if err != nil {
			return 0, err
		}
	case exchange.InternationalBankDepositFee:
		fee = getInternationalBankDepositFee(feeBuilder.Amount)
	case exchange.InternationalBankWithdrawalFee:
		fee = getInternationalBankWithdrawalFee(feeBuilder.Amount)
	case exchange.OfflineTradeFee:
		fee = getOfflineTradeFee(feeBuilder.PurchasePrice, feeBuilder.Amount)
	}
	if fee < 0 {
		fee = 0
	}
	return fee, nil
}

// getOfflineTradeFee calculates the worst case-scenario trading fee
// does not require an API request, requires manual updating
func getOfflineTradeFee(price, amount float64) float64 {
	return 0.001 * price * amount
}

// GetCryptocurrencyWithdrawalFee returns an estimate of fee based on type of transaction
func (b *Whitebit) GetCryptocurrencyWithdrawalFee(c currency.Code, accountFees AccountFees) (fee float64, err error) {
	switch result := accountFees.Withdraw[c.String()].(type) {
	case string:
		fee, err = strconv.ParseFloat(result, 64)
		if err != nil {
			return 0, err
		}
	case float64:
		fee = result
	}

	return fee, nil
}

func getInternationalBankDepositFee(amount float64) float64 {
	return 0.001 * amount
}

func getInternationalBankWithdrawalFee(amount float64) float64 {
	return 0.001 * amount
}

// CalculateTradingFee returns an estimate of fee based on type of whether is maker or taker fee
func (b *Whitebit) CalculateTradingFee(i []AccountInfo, purchasePrice, amount float64, c currency.Code, isMaker bool) (fee float64, err error) {
	for x := range i {
		for y := range i[x].Fees {
			if c.String() == i[x].Fees[y].Pairs {
				if isMaker {
					fee = i[x].Fees[y].MakerFees
				} else {
					fee = i[x].Fees[y].TakerFees
				}
				break
			}
		}
		if fee > 0 {
			break
		}
	}
	return (fee / 100) * purchasePrice * amount, err
}

// ConvertSymbolToWithdrawalType You need to have specific withdrawal types to withdraw from Whitebit
func (b *Whitebit) ConvertSymbolToWithdrawalType(c currency.Code) string {
	switch c {
	case currency.BTC:
		return "bitcoin"
	case currency.LTC:
		return "litecoin"
	case currency.ETH:
		return "ethereum"
	case currency.ETC:
		return "ethereumc"
	case currency.USDT:
		return "tetheruso"
	case currency.ZEC:
		return "zcash"
	case currency.XMR:
		return "monero"
	case currency.DSH:
		return "dash"
	case currency.XRP:
		return "ripple"
	case currency.SAN:
		return "santiment"
	case currency.OMG:
		return "omisego"
	case currency.BCH:
		return "bcash"
	case currency.ETP:
		return "metaverse"
	case currency.AVT:
		return "aventus"
	case currency.EDO:
		return "eidoo"
	case currency.BTG:
		return "bgold"
	case currency.DATA:
		return "datacoin"
	case currency.GNT:
		return "golem"
	case currency.SNT:
		return "status"
	default:
		return c.Lower().String()
	}
}

// ConvertSymbolToDepositMethod returns a converted currency deposit method
func (b *Whitebit) ConvertSymbolToDepositMethod(c currency.Code) (string, error) {
	if err := b.PopulateAcceptableMethods(); err != nil {
		return "", err
	}
	method, ok := AcceptableMethods[c.String()]
	if !ok {
		return "", fmt.Errorf("currency %s not supported in method list",
			c)
	}

	return strings.ToLower(method), nil
}

// PopulateAcceptableMethods retrieves all accepted currency strings and
// populates a map to check
func (b *Whitebit) PopulateAcceptableMethods() error {
	if len(AcceptableMethods) == 0 {
		var response [][][2]string
		err := b.SendHTTPRequest(exchange.RestSpot,
			whitebitAPIVersion2+whitebitDepositMethod,
			&response,
			configs)
		if err != nil {
			return err
		}

		if len(response) == 0 {
			return errors.New("response contains no data cannot populate acceptable method map")
		}

		for i := range response[0] {
			if len(response[0][i]) != 2 {
				return errors.New("response contains no data cannot populate acceptable method map")
			}
			AcceptableMethods[response[0][i][0]] = response[0][i][1]
		}
	}
	return nil
}

// GetWebsocketToken returns a websocket token
func (b *Whitebit) GetWebsocketToken() (string, error) {
	var response WsTokenResponse
	var err error
	if err = b.SendAuthenticatedHTTPRequestV2(exchange.RestSpot, http.MethodPost, witebitGetWsToken, nil, &response, orderMulti); err != nil {
		return "", err
	}

	return response.WebsocketToken, nil
}