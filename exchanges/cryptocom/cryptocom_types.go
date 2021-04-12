package cryptocom

import (
	"sync"
	"time"
)

const (
	goodTillCancel = "GOOD_TILL_CANCEL"
)

// FundingHistoryData stores funding history data
type FundingHistoryData struct {
	Time   int64   `json:"time"`
	Rate   float64 `json:"rate"`
	Symbol string  `json:"symbol"`
}

// OHLCV holds Open, High Low, Close, Volume data for set symbol
type OHLCV [][]float64

// Data stores last price for requested symbol
type Data struct {
	I  string `json:"i"`
	B  float64 `json:"b"`
	K  float64 `json:"k"`
	A  float64 `json:"a"`
	T  int64   `json:"t"`
	V  float64 `json:"v"`
	H  float64 `json:"h"`
	L  float64 `json:"l"`
	C  float64 `json:"c"`
}

// Instruments stores pair market data
type Tickers struct {
	Data       []Data  `json:"data"`
}

// TickersResp stores tickers market data
type TickersResp struct {
	Code              int64  `json:"code"`
	Result            Tickers  `json:"result"`
}

// SpotMarket stores market data
type SpotMarket struct {
	Symbol            string  `json:"symbol"`
	ID                string  `json:"id"`
	BaseCurrency      string  `json:"base_currency"`
	QuoteCurrency     string  `json:"quote_currency"`
	BaseMinSize       float64 `json:"base_min_size"`
	BaseMaxSize       float64 `json:"base_max_size"`
	BaseIncrementSize float64 `json:"base_increment_size"`
	QuoteMinPrice     float64 `json:"quote_min_price"`
	QuoteIncrement    float64 `json:"quote_increment"`
	Status            string  `json:"status"`
}

// Instrument stores pair market data
type Instrument struct {
	InstrumentName    string  `json:"instrument_name"`
	QuoteCurrency     string  `json:"quote_currency"`
	BaseCurrency      string  `json:"base_currency"`
	PriceDecimals     float64 `json:"price_decimals"`
	QuantityDecimals  float64 `json:"quantity_decimals"`
	MarginTradingEnabled       bool `json:"margin_trading_enabled"`
}

// Instruments stores pair market data
type Instruments struct {
	Instruments       []Instrument  `json:"instruments"`
}

// Instruments stores pairs market data
type InstrumentsResp struct {
	ID                int64  `json:"id"`
	Code              int64  `json:"code"`
	Result            Instruments  `json:"result"`
}

// Trade stores trade data
type Trade struct {
	SerialID int64   `json:"serialId"`
	Symbol   string  `json:"symbol"`
	Price    float64 `json:"price"`
	Amount   float64 `json:"size"`
	Time     int64   `json:"timestamp"`
	Side     string  `json:"side"`
	Type     string  `json:"type"`
}

// QuoteData stores quote data
type QuoteData struct {
	Price float64 `json:"price,string"`
	Size  float64 `json:"size,string"`
}

// Orderbook stores orderbook info
type Orderbook struct {
	Bids  [][]float64 `json:"bids"`
	Asks  [][]float64 `json:"asks"`
	T     int64       `json:"t"`
}

// Book stores pair market data
type Book struct {
	InstrumentName string `json:"instrument_name"`
	Depth      int64   `json:"depth"`
	Data       []Orderbook  `json:"data"`
}

// OrderbookResp stores orderbook data
type OrderbookResp struct {
	Code              int64  `json:"code"`
	Result            Book  `json:"result"`
}

// MarketStatistics stores market statistics for a particular product
type MarketStatistics struct {
	Open   float64   `json:"open,string"`
	Low    float64   `json:"low,string"`
	High   float64   `json:"high,string"`
	Close  float64   `json:"close,string"`
	Volume float64   `json:"volume,string"`
	Time   time.Time `json:"time"`
}

// ServerTime stores the server time data
type ServerTime struct {
	ISO   time.Time `json:"iso"`
	Epoch int64     `json:"epoch"`
}

// CurrencyBalance stores the account info data
type CurrencyBalance struct {
	Currency  string  `json:"currency"`
	Total     float64 `json:"total"`
	Available float64 `json:"available"`
}

// CurrencyBalance stores the account info data
type AccountSummary struct {
	Currency  string  `json:"currency"`
	Total     float64 `json:"total"`
	Available float64 `json:"available"`
}

type Bals struct {
	Balance  float64  `json:"balance"`
	Available  float64  `json:"available"`
	Order  float64  `json:"order"`
	Stake  float64  `json:"stake"`
	Currency  string  `json:"currency"`
}

type Summary struct {
	Accounts  []Bals `json:"accounts"`
}

type GetAccountSummary struct {
	ID     int64  `json:"id"`
	Method string `json:"method"`
	Code   int64  `json:"code"`
	Result Summary `json:"result"`
}

// AccountFees stores fee for each currency pair
type AccountFees struct {
	MakerFee float64 `json:"makerFee"`
	Symbol   string  `json:"symbol"`
	TakerFee float64 `json:"takerFee"`
}

// TradeHistory stores user trades for exchange
type TradeHistory []struct {
	Base         string  `json:"base"`
	ClOrderID    string  `json:"clOrderID"`
	FeeAmount    float64 `json:"feeAmount"`
	FeeCurrency  string  `json:"feeCurrency"`
	FilledPrice  float64 `json:"filledPrice"`
	FilledSize   float64 `json:"filledSize"`
	OrderID      string  `json:"orderId"`
	OrderType    int     `json:"orderType"`
	Price        float64 `json:"price"`
	Quote        string  `json:"quote"`
	RealizedPnl  float64 `json:"realizedPnl"`
	SerialID     int64   `json:"serialId"`
	Side         string  `json:"side"`
	Size         float64 `json:"size"`
	Symbol       string  `json:"symbol"`
	Timestamp    string  `json:"timestamp"`
	Total        float64 `json:"total"`
	TradeID      string  `json:"tradeId"`
	TriggerPrice float64 `json:"triggerPrice"`
	TriggerType  int     `json:"triggerType"`
	Username     string  `json:"username"`
	Wallet       string  `json:"wallet"`
}

// WalletHistory stores account funding history
type WalletHistory []struct {
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	Description string  `json:"description"`
	Fees        float64 `json:"fees"`
	OrderID     string  `json:"orderId"`
	Status      string  `json:"status"`
	Timestamp   int64   `json:"timestamp"`
	Type        string  `json:"type"`
	Username    string  `json:"username"`
	Wallet      string  `json:"wallet"`
}

// WalletAddress stores address for crypto deposit's
type WalletAddress []struct {
	Address string `json:"address"`
	Created int    `json:"created"`
}

// WithdrawalResponse response received when submitting a crypto withdrawal request
type WithdrawalResponse struct {
	WithdrawID string `json:"withdraw_id"`
}

type Order struct {
	Status  string  `json:"status"`
	Side  string  `json:"side"`
	Price  float64  `json:"price"`
	Quantity  float64  `json:"quantity"`
	OrderId  string  `json:"order_id"`
	ClientOid  string  `json:"client_oid"`
	CreateTime  int64  `json:"create_time"`
	UpdateTime  int64  `json:"update_time"`
	Type  string  `json:"type"`
	InstrumentName  string  `json:"instrument_name"`
	CumulativeQuantity  float64  `json:"cumulative_quantity"`
	CumulativeValue  float64  `json:"cumulative_value"`
	AvgPrice  float64  `json:"avg_price"`
	FeeCurrency  string  `json:"fee_currency"`
	TimeInForce  string  `json:"time_in_force"`

	// user.order  subscriptions
	Reason  string  `json:"reason"`
	ExecInst  string  `json:"exec_inst"`
	TriggerPrice  float64  `json:"trigger_price"`
}

type OpenOrders struct {
	Count  int64  `json:"count"`
	OrderList  []Order `json:"order_list"`
}

type GetOpenOrders struct {
	ID     int64  `json:"id"`
	Method string `json:"method"`
	Code   int64  `json:"code"`
	Result OpenOrders `json:"result"`
}

// CancelOrder stores slice of orders
//type CancelOrder []Order
type CancelOrder struct {
	ID     int64  `json:"id"`
	Method string `json:"method"`
	Code   int64  `json:"code"`
	Success bool
}

type CreateOrder struct {
	OrderID  string  `json:"order_id"`
	ClientOID string `json:"client_oid"`
	OrderPlaced bool `json:"order_placed"`
}

type CreateOrderResp struct {
	ID     int64  `json:"id"`
	Method string `json:"method"`
	Code   int64  `json:"code"`
	Result CreateOrder `json:"result"`
}

type wsSub struct {
	ID   int64    `json:"id"`
	Method string `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
	Nonce int64   `json:"nonce"`
	ApiKey string   `json:"api_key,omitempty"`
	Sig string   `json:"sig,omitempty"`
}

type wsAuth struct {
	ID   int64    `json:"id"`
	Method string `json:"method"`
	Nonce int64   `json:"nonce"`
	ApiKey string   `json:"api_key"`
	Sig string   `json:"sig"`
}

type Result struct {
	InstrumentName string   `json:"instrument_name"`
	Subscription string   `json:"subscription"`
	Channel string   `json:"channel"`
	Data  []interface{} `json:"data"`
}

type WsSubRead struct {
	ID     int64  `json:"id"`
	Method string `json:"method"`
	Code   int64  `json:"code"`
	Result Result `json:"result"`
}

type ResultBook struct {
	InstrumentName string   `json:"instrument_name"`
	Subscription string   `json:"subscription"`
	Channel string   `json:"channel"`
	Data  []Orderbook `json:"data"`
}

type WsReadOrderBook struct {
	Method string   `json:"method"`
	Code   int64  `json:"code"`
	Result ResultBook `json:"result"`
}

// Ticker stores Ticker info
type Ticker struct {
	H  float64 `json:"h"` // Price of the 24h highest trade
	V  float64 `json:"v"` // The total 24h traded volume
	A  float64 `json:"a"` // The price of the latest trade, null if there weren't any trades
	L  float64 `json:"l"` // Price of the 24h lowest trade, null if there weren't any trades
	B  float64 `json:"b"` // The current best bid price, null if there aren't any bids
	K  float64 `json:"k"` // The current best ask price, null if there aren't any asks
	C  float64 `json:"c"` // 24-hour price change, null if there weren't any trades
	T  int64   `json:"t"` // update time
}

type ReadTicker struct {
	InstrumentName string   `json:"instrument_name"`
	Subscription string   `json:"subscription"`
	Channel string   `json:"channel"`
	Data  []Ticker `json:"data"`
}

type WsReadTicker struct {
	Method string   `json:"method"`
	Result ReadTicker `json:"result"`
}

type wsQuoteData struct {
	Total string `json:"cumulativeTotal"`
	Price string `json:"price"`
	Size  string `json:"size"`
}

type wsOBData struct {
	Currency  string        `json:"currency"`
	BuyQuote  []wsQuoteData `json:"buyQuote"`
	SellQuote []wsQuoteData `json:"sellQuote"`
}

type wsOrderBook struct {
	Topic string   `json:"topic"`
	Data  wsOBData `json:"data"`
}

type wsTradeData struct {
	Side            string  `json:"side"`
	InstrumentName  string  `json:"instrument_name"`
	Fee             float64 `json:"fee"`
	TradeId         string  `json:"trade_id"`
	CreateTime      int64   `json:"create_time,string"` // check if no float64
	TradedPrice     float64 `json:"traded_price"`
	TradedQuantity  float64 `json:"traded_quantity"`
	FeeCurrency     string  `json:"fee_currency"`
	OrderId     	string  `json:"order_id"`

	//Amount          float64 `json:"amount"`
	//Gain            int64   `json:"gain"`
	//Newest          int64   `json:"newest"`
	//Price           float64 `json:"price"`
	//ID              int64   `json:"serialId"`
	//TransactionTime int64   `json:"transactionUnixTime"`
}

type wsTradeHistory struct {
	Topic string        `json:"topic"`
	Data  []wsTradeData `json:"data"`
}

type wsNotification struct {
	Topic string          `json:"topic"`
	Data  []wsOrderUpdate `json:"data"`
}

type wsOrderUpdate struct {
	OrderID           string  `json:"orderID"`
	OrderMode         string  `json:"orderMode"`
	OrderType         string  `json:"orderType"`
	PegPriceDeviation string  `json:"pegPriceDeviation"`
	Price             float64 `json:"price,string"`
	Size              float64 `json:"size,string"`
	Status            string  `json:"status"`
	Stealth           string  `json:"stealth"`
	Symbol            string  `json:"symbol"`
	Timestamp         int64   `json:"timestamp,string"`
	TriggerPrice      float64 `json:"triggerPrice,string"`
	Type              string  `json:"type"`
}

// ErrorResponse contains errors received from API
type ErrorResponse struct {
	ErrorCode int    `json:"errorCode"`
	Message   string `json:"message"`
	Status    int    `json:"status"`
}

// OrderSizeLimit holds accepted minimum, maximum, and size increment when submitting new orders
type OrderSizeLimit struct {
	MinOrderSize     float64
	MaxOrderSize     float64
	MinSizeIncrement float64
}

// orderSizeLimitMap map of OrderSizeLimit per currency
var orderSizeLimitMap sync.Map

// WsSubscriptionAcknowledgement contains successful subscription messages
type WsSubscriptionAcknowledgement struct {
	Channel []string `json:"channel"`
	Event   string   `json:"event"`
}

// WsLoginAcknowledgement contains whether authentication was successful
type WsLoginAcknowledgement struct {
	Event   string `json:"event"`
	Success bool   `json:"success"`
}


type DetailResult struct {
	TradeList []TradeData `json:"trade_list"`
	OrderInfo Order `json:"order_info"`
}

type OrderDetail struct {
	ID     int64  `json:"id"`
	Method string `json:"method"`
	Code   int64  `json:"code"`
	Result DetailResult `json:"result"`
}

type TradeData struct {
	Side            string  `json:"side"`
	InstrumentName  string  `json:"instrument_name"`
	Fee             float64 `json:"fee"`
	TradeId         string  `json:"trade_id"`
	CreateTime      int64   `json:"create_time"`
	TradedPrice     float64 `json:"traded_price"`
	TradedQuantity  float64 `json:"traded_quantity"`
	FeeCurrency     string  `json:"fee_currency"`
	OrderId     	string  `json:"order_id"`
}

type UserOrderResponse struct {
	OrderId     	string  `json:"order_id"`
	Status          string  `json:"status"`
	Reason          float64  `json:"reason"`
}

