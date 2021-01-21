package engine

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/vazha/gocryptotrader/common"
	"github.com/vazha/gocryptotrader/currency"
	"github.com/vazha/gocryptotrader/exchanges/order"
	"github.com/vazha/gocryptotrader/exchanges/orderbook"
	"github.com/vazha/gocryptotrader/exchanges/stats"
	"github.com/vazha/gocryptotrader/exchanges/stream"
	"github.com/vazha/gocryptotrader/exchanges/ticker"
	"github.com/vazha/gocryptotrader/log"
)

func printCurrencyFormat(price float64) string {
	displaySymbol, err := currency.GetSymbolByCurrencyName(Bot.Config.Currency.FiatDisplayCurrency)
	if err != nil {
		log.Errorf(log.Global, "Failed to get display symbol: %s\n", err)
	}

	return fmt.Sprintf("%s%.8f", displaySymbol, price)
}

func printConvertCurrencyFormat(origCurrency currency.Code, origPrice float64) string {
	displayCurrency := Bot.Config.Currency.FiatDisplayCurrency
	conv, err := currency.ConvertCurrency(origPrice,
		origCurrency,
		displayCurrency)
	if err != nil {
		log.Errorf(log.Global, "Failed to convert currency: %s\n", err)
	}

	displaySymbol, err := currency.GetSymbolByCurrencyName(displayCurrency)
	if err != nil {
		log.Errorf(log.Global, "Failed to get display symbol: %s\n", err)
	}

	origSymbol, err := currency.GetSymbolByCurrencyName(origCurrency)
	if err != nil {
		log.Errorf(log.Global, "Failed to get original currency symbol for %s: %s\n",
			origCurrency,
			err)
	}

	return fmt.Sprintf("%s%.2f %s (%s%.2f %s)",
		displaySymbol,
		conv,
		displayCurrency,
		origSymbol,
		origPrice,
		origCurrency,
	)
}

func printTickerSummary(result *ticker.Price, protocol string, err error) {
	if err != nil {
		if err == common.ErrNotYetImplemented {
			log.Warnf(log.Ticker, "Failed to get %s ticker. Error: %s\n",
				protocol,
				err)
			return
		}
		log.Errorf(log.Ticker, "Failed to get %s ticker. Error: %s\n",
			protocol,
			err)
		return
	}

	stats.Add(result.ExchangeName, result.Pair, result.AssetType, result.Last, result.Volume)
	if result.Pair.Quote.IsFiatCurrency() &&
		result.Pair.Quote != Bot.Config.Currency.FiatDisplayCurrency {
		origCurrency := result.Pair.Quote.Upper()
		log.Infof(log.Ticker, "%s %s %s %s: TICKER: Last %s Ask %s Bid %s High %s Low %s Volume %.8f\n",
			result.ExchangeName,
			protocol,
			FormatCurrency(result.Pair),
			strings.ToUpper(result.AssetType.String()),
			printConvertCurrencyFormat(origCurrency, result.Last),
			printConvertCurrencyFormat(origCurrency, result.Ask),
			printConvertCurrencyFormat(origCurrency, result.Bid),
			printConvertCurrencyFormat(origCurrency, result.High),
			printConvertCurrencyFormat(origCurrency, result.Low),
			result.Volume)
	} else {
		if result.Pair.Quote.IsFiatCurrency() &&
			result.Pair.Quote == Bot.Config.Currency.FiatDisplayCurrency {
			log.Infof(log.Ticker, "%s %s %s %s: TICKER: Last %s Ask %s Bid %s High %s Low %s Volume %.8f\n",
				result.ExchangeName,
				protocol,
				FormatCurrency(result.Pair),
				strings.ToUpper(result.AssetType.String()),
				printCurrencyFormat(result.Last),
				printCurrencyFormat(result.Ask),
				printCurrencyFormat(result.Bid),
				printCurrencyFormat(result.High),
				printCurrencyFormat(result.Low),
				result.Volume)
		} else {
			log.Infof(log.Ticker, "%s %s %s %s: TICKER: Last %.8f Ask %.8f Bid %.8f High %.8f Low %.8f Volume %.8f\n",
				result.ExchangeName,
				protocol,
				FormatCurrency(result.Pair),
				strings.ToUpper(result.AssetType.String()),
				result.Last,
				result.Ask,
				result.Bid,
				result.High,
				result.Low,
				result.Volume)
		}
	}
}

const (
	book = "%s %s %s %s: ORDERBOOK: Bids len: %d Amount: %f %s. Total value: %s Asks len: %d Amount: %f %s. Total value: %s\n"
)

func printOrderbookSummary(result *orderbook.Base, protocol string, err error) {
	if err != nil {
		if result == nil {
			log.Errorf(log.OrderBook, "Failed to get %s orderbook. Error: %s\n",
				protocol,
				err)
			return
		}
		if err == common.ErrNotYetImplemented {
			log.Warnf(log.OrderBook, "Failed to get %s orderbook for %s %s %s. Error: %s\n",
				protocol,
				result.ExchangeName,
				result.Pair,
				result.AssetType,
				err)
			return
		}
		log.Errorf(log.OrderBook, "Failed to get %s orderbook for %s %s %s. Error: %s\n",
			protocol,
			result.ExchangeName,
			result.Pair,
			result.AssetType,
			err)
		return
	}

	bidsAmount, bidsValue := result.TotalBidsAmount()
	asksAmount, asksValue := result.TotalAsksAmount()

	var bidValueResult, askValueResult string
	switch {
	case result.Pair.Quote.IsFiatCurrency() && result.Pair.Quote != Bot.Config.Currency.FiatDisplayCurrency:
		origCurrency := result.Pair.Quote.Upper()
		bidValueResult = printConvertCurrencyFormat(origCurrency, bidsValue)
		askValueResult = printConvertCurrencyFormat(origCurrency, asksValue)
	case result.Pair.Quote.IsFiatCurrency() && result.Pair.Quote == Bot.Config.Currency.FiatDisplayCurrency:
		bidValueResult = printCurrencyFormat(bidsValue)
		askValueResult = printCurrencyFormat(asksValue)
	default:
		bidValueResult = strconv.FormatFloat(bidsValue, 'f', -1, 64)
		askValueResult = strconv.FormatFloat(asksValue, 'f', -1, 64)
	}
	log.Infof(log.OrderBook, book,
		result.ExchangeName,
		protocol,
		FormatCurrency(result.Pair),
		strings.ToUpper(result.AssetType.String()),
		len(result.Bids),
		bidsAmount,
		result.Pair.Base,
		bidValueResult,
		len(result.Asks),
		asksAmount,
		result.Pair.Base,
		askValueResult,
	)
}

func relayWebsocketEvent(result interface{}, event, assetType, exchangeName string) {
	evt := WebsocketEvent{
		Data:      result,
		Event:     event,
		AssetType: assetType,
		Exchange:  exchangeName,
	}
	err := BroadcastWebsocketMessage(evt)
	if err != nil {
		log.Errorf(log.WebsocketMgr, "Failed to broadcast websocket event %v. Error: %s\n",
			event, err)
	}
}

// WebsocketRoutine Initial routine management system for websocket
func (bot *Engine) WebsocketRoutine() {
	if bot.Settings.Verbose {
		log.Debugln(log.WebsocketMgr, "Connecting exchange websocket services...")
	}

	exchanges := bot.GetExchanges()
	for i := range exchanges {
		go func(i int) {
			if exchanges[i].SupportsWebsocket() {
				if bot.Settings.Verbose {
					log.Debugf(log.WebsocketMgr,
						"Exchange %s websocket support: Yes Enabled: %v\n",
						exchanges[i].GetName(),
						common.IsEnabled(exchanges[i].IsWebsocketEnabled()),
					)
				}

				ws, err := exchanges[i].GetWebsocket()
				if err != nil {
					log.Errorf(
						log.WebsocketMgr,
						"Exchange %s GetWebsocket error: %s\n",
						exchanges[i].GetName(),
						err,
					)
					return
				}

				// Exchange sync manager might have already started ws
				// service or is in the process of connecting, so check
				if ws.IsConnected() || ws.IsConnecting() {
					return
				}

				// Data handler routine
				go bot.WebsocketDataReceiver(ws)

				if ws.IsEnabled() {
					err = ws.Connect()
					if err != nil {
						log.Errorf(log.WebsocketMgr, "%v\n", err)
					}
					err = ws.FlushChannels()
					if err != nil {
						log.Errorf(log.WebsocketMgr, "Failed to subscribe: %v\n", err)
					}
				}
			} else if bot.Settings.Verbose {
				log.Debugf(log.WebsocketMgr,
					"Exchange %s websocket support: No\n",
					exchanges[i].GetName(),
				)
			}
		}(i)
	}
}

var shutdowner = make(chan struct{}, 1)
var wg sync.WaitGroup

// WebsocketDataReceiver handles websocket data coming from a websocket feed
// associated with an exchange
func (bot *Engine) WebsocketDataReceiver(ws *stream.Websocket) {
	wg.Add(1)
	defer wg.Done()

	for {
		select {
		case <-shutdowner:
			return
		case data := <-ws.ToRoutine:
			err := bot.WebsocketDataHandler(ws.GetName(), data)
			if err != nil {
				log.Error(log.WebsocketMgr, err)
			}
		}
	}
}

// WebsocketDataHandler is a central point for exchange websocket implementations to send
// processed data. WebsocketDataHandler will then pass that to an appropriate handler
func (bot *Engine) WebsocketDataHandler(exchName string, data interface{}) error {
	if data == nil {
		return fmt.Errorf("routines.go - exchange %s nil data sent to websocket",
			exchName)
	}

	switch d := data.(type) {
	case string:
		log.Info(log.WebsocketMgr, d)
	case error:
		return fmt.Errorf("routines.go exchange %s websocket error - %s", exchName, data)
	case stream.FundingData:
		if bot.Settings.Verbose {
			log.Infof(log.WebsocketMgr, "%s websocket %s %s funding updated %+v",
				exchName,
				FormatCurrency(d.CurrencyPair),
				d.AssetType,
				d)
		}
	case *ticker.Price:
		if bot.Settings.EnableExchangeSyncManager && bot.ExchangeCurrencyPairManager != nil {
			bot.ExchangeCurrencyPairManager.update(exchName,
				d.Pair,
				d.AssetType,
				SyncItemTicker,
				nil)
		}
		err := ticker.ProcessTicker(d)
		printTickerSummary(d, "websocket", err)
	case stream.KlineData:
		if bot.Settings.Verbose {
			log.Infof(log.WebsocketMgr, "%s websocket %s %s kline updated %+v",
				exchName,
				FormatCurrency(d.Pair),
				d.AssetType,
				d)
		}
	case *orderbook.Base:
		if bot.Settings.EnableExchangeSyncManager && bot.ExchangeCurrencyPairManager != nil {
			bot.ExchangeCurrencyPairManager.update(exchName,
				d.Pair,
				d.AssetType,
				SyncItemOrderbook,
				nil)
		}
		printOrderbookSummary(d, "websocket", nil)
	case *order.Detail:
		if !bot.OrderManager.orderStore.exists(d) {
			err := bot.OrderManager.orderStore.Add(d)
			if err != nil {
				return err
			}
		} else {
			od, err := bot.OrderManager.orderStore.GetByExchangeAndID(d.Exchange, d.ID)
			if err != nil {
				return err
			}
			od.UpdateOrderFromDetail(d)
		}
	case *order.Cancel:
		return bot.OrderManager.Cancel(d)
	case *order.Modify:
		od, err := bot.OrderManager.orderStore.GetByExchangeAndID(d.Exchange, d.ID)
		if err != nil {
			return err
		}
		od.UpdateOrderFromModify(d)
	case order.ClassificationError:
		return errors.New(d.Error())
	case stream.UnhandledMessageWarning:
		log.Warn(log.WebsocketMgr, d.Message)
	default:
		if bot.Settings.Verbose {
			log.Warnf(log.WebsocketMgr,
				"%s websocket Unknown type: %+v",
				exchName,
				d)
		}
	}
	return nil
}
