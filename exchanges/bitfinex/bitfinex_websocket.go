package bitfinex

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vazha/gocryptotrader/common/crypto"
	"github.com/vazha/gocryptotrader/currency"
	exchange "github.com/vazha/gocryptotrader/exchanges"
	"github.com/vazha/gocryptotrader/exchanges/asset"
	"github.com/vazha/gocryptotrader/exchanges/order"
	"github.com/vazha/gocryptotrader/exchanges/orderbook"
	"github.com/vazha/gocryptotrader/exchanges/ticker"
	"github.com/vazha/gocryptotrader/exchanges/websocket/wshandler"
	"github.com/vazha/gocryptotrader/exchanges/websocket/wsorderbook"
	log "github.com/vazha/gocryptotrader/logger"
)

var comms = make(chan wshandler.WebsocketResponse)

// WsConnect starts a new websocket connection
func (b *Bitfinex) WsConnect() error {
	if !b.Websocket.IsEnabled() || !b.IsEnabled() {
		return errors.New(wshandler.WebsocketNotEnabled)
	}

	var dialer websocket.Dialer
	err := b.WebsocketConn.Dial(&dialer, http.Header{})
	if err != nil {
		return fmt.Errorf("%v unable to connect to Websocket. Error: %s", b.Name, err)
	}
	go b.WsReadData(b.WebsocketConn)

	if b.Websocket.CanUseAuthenticatedEndpoints() {
		err = b.AuthenticatedWebsocketConn.Dial(&dialer, http.Header{})
		if err != nil {
			log.Errorf(log.ExchangeSys, "%v unable to connect to authenticated Websocket. Error: %s", b.Name, err)
			b.Websocket.SetCanUseAuthenticatedEndpoints(false)
		}
		go b.WsReadData(b.AuthenticatedWebsocketConn)
		err = b.WsSendAuth()
		if err != nil {
			log.Errorf(log.ExchangeSys, "%v - authentication failed: %v\n", b.Name, err)
			b.Websocket.SetCanUseAuthenticatedEndpoints(false)
		}
	}

	b.GenerateDefaultSubscriptions()
	go b.WsDataHandler()
	return nil
}

// WsReadData funnels both auth and public ws data into one manageable place
func (b *Bitfinex) WsReadData(ws *wshandler.WebsocketConnection) {
	b.Websocket.Wg.Add(1)
	defer b.Websocket.Wg.Done()
	for {
		select {
		case <-b.Websocket.ShutdownC:
			return
		default:
			resp, err := ws.ReadMessage()
			if err != nil {
				b.Websocket.DataHandler <- err
				return
			}
			b.Websocket.TrafficAlert <- struct{}{}
			comms <- resp
		}
	}
}

// WsDataHandler handles data from WsReadData
func (b *Bitfinex) WsDataHandler() {
	b.Websocket.Wg.Add(1)
	defer b.Websocket.Wg.Done()

	for {
		select {
		case <-b.Websocket.ShutdownC:
			return
		case stream := <-comms:
			if stream.Type == websocket.TextMessage {
				var result interface{}
				err := json.Unmarshal(stream.Raw, &result)
				if err != nil {
					b.Websocket.DataHandler <- err
					return
				}
				switch reflect.TypeOf(result).String() {
				case "map[string]interface {}":
					eventData := result.(map[string]interface{})
					event := eventData["event"]
					switch event {
					case "subscribed":
						if symbol, ok := eventData["pair"].(string); ok {
							b.WsAddSubscriptionChannel(int(eventData["chanId"].(float64)),
								eventData["channel"].(string),
								symbol,
							)
						} else if key, ok := eventData["key"].(string); ok {
							b.WsAddSubscriptionChannel(int(eventData["chanId"].(float64)),
								eventData["channel"].(string),
								key,
							)
						}
					case "auth":
						status := eventData["status"].(string)
						if status == "OK" {
							b.Websocket.DataHandler <- eventData
							b.WsAddSubscriptionChannel(0, "account", "N/A")
						} else if status == "fail" {
							b.Websocket.DataHandler <- fmt.Errorf("bitfinex.go error - Websocket unable to AUTH. Error code: %s",
								eventData["code"].(string))
						}
					}
				case "[]interface {}":
					chanData := result.([]interface{})
					if hb, ok := chanData[1].(string); ok {
						// Capturing heart beat
						if hb == "hb" {
							continue
						}
					}
					chanID := int(chanData[0].(float64))
					chanInfo, ok := b.WebsocketSubdChannels[chanID]
					if !ok && chanID != 0 {
						b.Websocket.DataHandler <- fmt.Errorf("bitfinex.go error - Unable to locate chanID: %d",
							chanID)
						continue
					}

					switch chanInfo.Channel {
					case wsBook:
						var newOrderbook []WebsocketBook
						curr := currency.NewPairFromString(chanInfo.Pair)
						if obSnapBundle, ok := chanData[1].([]interface{}); ok {
							switch id := obSnapBundle[0].(type) {
							case []interface{}:
								for i := range obSnapBundle {
									data := obSnapBundle[i].([]interface{})
									newOrderbook = append(newOrderbook, WebsocketBook{
										ID:     int64(data[0].(float64)),
										Price:  data[1].(float64),
										Amount: data[2].(float64)})
								}
								err := b.WsInsertSnapshot(curr,
									asset.Spot,
									newOrderbook)
								if err != nil {
									b.Websocket.DataHandler <- fmt.Errorf("bitfinex_websocket.go inserting snapshot error: %s",
										err)
								}
							case float64:
								newOrderbook = append(newOrderbook, WebsocketBook{
									ID:     int64(id),
									Price:  obSnapBundle[1].(float64),
									Amount: obSnapBundle[2].(float64)})
								err := b.WsUpdateOrderbook(curr,
									asset.Spot,
									newOrderbook)
								if err != nil {
									b.Websocket.DataHandler <- fmt.Errorf("bitfinex_websocket.go inserting snapshot error: %s",
										err)
								}
							}
						}
						continue
					case wsCandles:
						curr := currency.NewPairFromString(chanInfo.Pair)
						if candleBundle, ok := chanData[1].([]interface{}); ok {
							if len(candleBundle) == 0 {
								continue
							}
							switch candleBundle[0].(type) {
							case []interface{}:
								for i := range candleBundle {
									candle := candleBundle[i].([]interface{})
									b.Websocket.DataHandler <- wshandler.KlineData{
										Timestamp:  time.Unix(0, candle[0].(int64)),
										Exchange:   b.Name,
										AssetType:  asset.Spot,
										Pair:       curr,
										OpenPrice:  candle[1].(float64),
										ClosePrice: candle[2].(float64),
										HighPrice:  candle[3].(float64),
										LowPrice:   candle[4].(float64),
										Volume:     candle[5].(float64),
									}
								}
							case float64:
								b.Websocket.DataHandler <- wshandler.KlineData{
									Timestamp:  time.Unix(0, candleBundle[0].(int64)),
									Exchange:   b.Name,
									AssetType:  asset.Spot,
									Pair:       curr,
									OpenPrice:  candleBundle[1].(float64),
									ClosePrice: candleBundle[2].(float64),
									HighPrice:  candleBundle[3].(float64),
									LowPrice:   candleBundle[4].(float64),
									Volume:     candleBundle[5].(float64),
								}
							}
						}
						continue
					case wsTicker:
						tickerData := chanData[1].([]interface{})
						b.Websocket.DataHandler <- &ticker.Price{
							ExchangeName: b.Name,
							Bid:          tickerData[0].(float64),
							Ask:          tickerData[2].(float64),
							Last:         tickerData[6].(float64),
							Volume:       tickerData[7].(float64),
							High:         tickerData[8].(float64),
							Low:          tickerData[9].(float64),
							AssetType:    asset.Spot,
							Pair:         currency.NewPairFromString(chanInfo.Pair),
						}
						continue
					case wsTrades:
						var trades []WebsocketTrade
						switch len(chanData) {
						case 2:
							snapshot := chanData[1].([]interface{})
							for i := range snapshot {
								elem := snapshot[i].([]interface{})
								if len(elem) == 5 {
									trades = append(trades,
										WebsocketTrade{
											ID:        int64(elem[0].(float64)),
											Timestamp: int64(elem[1].(float64)),
											Amount:    elem[3].(float64),
											Rate:      elem[4].(float64),
											Period:    int64(elem[4].(float64)),
										})
									continue
								}
								trades = append(trades,
									WebsocketTrade{
										ID:        int64(elem[0].(float64)),
										Timestamp: int64(elem[1].(float64)),
										Price:     elem[3].(float64),
										Amount:    elem[2].(float64),
									})
							}
						case 3:
							if chanData[1].(string) == wsTradeExecutionUpdate ||
								chanData[1].(string) == wsFundingTradeUpdate {
								// "(f)te - trade executed" && "(f)tu - trade updated"
								// contain the same amount of data
								// "(f)te" gets sent first so we can drop "(f)tu"
								continue
							}
							data := chanData[2].([]interface{})
							trades = append(trades, WebsocketTrade{
								ID:        int64(data[0].(float64)),
								Timestamp: int64(data[1].(float64)),
								Price:     data[3].(float64),
								Amount:    data[2].(float64)})
						}

						for i := range trades {
							side := order.Buy.String()
							newAmount := trades[i].Amount
							if newAmount < 0 {
								side = order.Sell.String()
								newAmount *= -1
							}

							if trades[i].Rate > 0 {
								b.Websocket.DataHandler <- wshandler.FundingData{
									CurrencyPair: currency.NewPairFromString(chanInfo.Pair),
									Timestamp:    time.Unix(0, trades[i].Timestamp*int64(time.Millisecond)),
									Amount:       newAmount,
									Exchange:     b.Name,
									AssetType:    asset.Spot,
									Side:         side,
									Rate:         trades[i].Rate,
									Period:       trades[i].Period,
								}
								continue
							}

							b.Websocket.DataHandler <- wshandler.TradeData{
								CurrencyPair: currency.NewPairFromString(chanInfo.Pair),
								Timestamp:    time.Unix(0, trades[i].Timestamp*int64(time.Millisecond)),
								Price:        trades[i].Price,
								Amount:       newAmount,
								Exchange:     b.Name,
								AssetType:    asset.Spot,
								Side:         side,
							}
						}
						continue
					}

					if authResp, ok := chanData[1].(string); ok {
						switch authResp {
						case wsHeartbeat, pong:
							continue
						case wsNotification:
							notification := chanData[2].([]interface{})
							if data, ok := notification[4].([]interface{}); ok {
								channelName := notification[1].(string)
								switch {
								case strings.Contains(channelName, wsOrderUpdate),
									strings.Contains(channelName, wsOrderCancel),
									strings.Contains(channelName, wsFundingOrderCancel):
									if data[0] != nil && data[0].(float64) > 0 {
										b.AuthenticatedWebsocketConn.AddResponseWithID(int64(data[0].(float64)), stream.Raw)
										continue
									}
								case strings.Contains(channelName, wsOrderNew):
									if data[2] != nil && data[2].(float64) > 0 {
										b.AuthenticatedWebsocketConn.AddResponseWithID(int64(data[2].(float64)), stream.Raw)
										continue
									}
								}
								b.Websocket.DataHandler <- fmt.Errorf("%s - Unexpected data returned %s", b.Name, stream.Raw)
								continue
							}
							if notification[5] != nil && strings.EqualFold(notification[5].(string), wsError) {
								b.Websocket.DataHandler <- fmt.Errorf("%s - Error %s", b.Name, notification[6].(string))
							}
						case wsPositionSnapshot:
							var snapshot []WebsocketPosition
							if snapBundle, ok := chanData[2].([]interface{}); ok && len(snapBundle) > 0 {
								if _, ok := snapBundle[0].([]interface{}); ok {
									for i := range snapBundle {
										positionData := snapBundle[i].([]interface{})
										position := WebsocketPosition{
											Pair:              positionData[0].(string),
											Status:            positionData[1].(string),
											Amount:            positionData[2].(float64),
											Price:             positionData[3].(float64),
											MarginFunding:     positionData[4].(float64),
											MarginFundingType: int64(positionData[5].(float64)),
											ProfitLoss:        positionData[6].(float64),
											ProfitLossPercent: positionData[7].(float64),
											LiquidationPrice:  positionData[8].(float64),
											Leverage:          positionData[9].(float64),
										}
										snapshot = append(snapshot, position)
									}
									b.Websocket.DataHandler <- snapshot
								}
							}
						case wsPositionNew, wsPositionUpdate, wsPositionClose:
							if positionData, ok := chanData[2].([]interface{}); ok && len(positionData) > 0 {
								position := WebsocketPosition{
									Pair:              positionData[0].(string),
									Status:            positionData[1].(string),
									Amount:            positionData[2].(float64),
									Price:             positionData[3].(float64),
									MarginFunding:     positionData[4].(float64),
									MarginFundingType: int64(positionData[5].(float64)),
									ProfitLoss:        positionData[6].(float64),
									ProfitLossPercent: positionData[7].(float64),
									LiquidationPrice:  positionData[8].(float64),
									Leverage:          positionData[9].(float64),
								}
								b.Websocket.DataHandler <- position
							}
						case wsTradeExecutionUpdate:
							if tradeData, ok := chanData[2].([]interface{}); ok && len(tradeData) > 4 {
								b.Websocket.DataHandler <- WebsocketTradeData{
									TradeID:        int64(tradeData[0].(float64)),
									Pair:           tradeData[1].(string),
									Timestamp:      int64(tradeData[2].(float64)),
									OrderID:        int64(tradeData[3].(float64)),
									AmountExecuted: tradeData[4].(float64),
									PriceExecuted:  tradeData[5].(float64),
									OrderType:      tradeData[6].(string),
									OrderPrice:     tradeData[7].(float64),
									Maker:          tradeData[8].(float64) == 1,
									Fee:            tradeData[9].(float64),
									FeeCurrency:    tradeData[10].(string),
								}
							}
						case wsFundingOrderSnapshot:
							var snapshot []WsFundingOffer
							if snapBundle, ok := chanData[2].([]interface{}); ok && len(snapBundle) > 0 {
								if _, ok := snapBundle[0].([]interface{}); ok {
									for i := range snapBundle {
										data := snapBundle[i].([]interface{})
										offer := WsFundingOffer{
											ID:         int64(data[0].(float64)),
											Symbol:     data[1].(string),
											Created:    int64(data[2].(float64)),
											Updated:    int64(data[3].(float64)),
											Amount:     data[4].(float64),
											AmountOrig: data[5].(float64),
											Type:       data[6].(string),
											Flags:      data[9].(float64),
											Status:     data[10].(string),
											Rate:       data[14].(float64),
											Period:     int64(data[15].(float64)),
											Notify:     data[16].(float64) == 1,
											Hidden:     data[17].(float64) == 1,
											Insure:     data[18].(float64) == 1,
											Renew:      data[19].(float64) == 1,
											RateReal:   data[20].(float64),
										}
										snapshot = append(snapshot, offer)
									}
									b.Websocket.DataHandler <- snapshot
								}
							}
						case wsFundingOrderNew, wsFundingOrderUpdate, wsFundingOrderCancel:
							if data, ok := chanData[2].([]interface{}); ok && len(data) > 0 {
								b.Websocket.DataHandler <- WsFundingOffer{
									ID:         int64(data[0].(float64)),
									Symbol:     data[1].(string),
									Created:    int64(data[2].(float64)),
									Updated:    int64(data[3].(float64)),
									Amount:     data[4].(float64),
									AmountOrig: data[5].(float64),
									Type:       data[6].(string),
									Flags:      data[9].(float64),
									Status:     data[10].(string),
									Rate:       data[14].(float64),
									Period:     int64(data[15].(float64)),
									Notify:     data[16].(float64) == 1,
									Hidden:     data[17].(float64) == 1,
									Insure:     data[18].(float64) == 1,
									Renew:      data[19].(float64) == 1,
									RateReal:   data[20].(float64),
								}
							}
						case wsFundingCreditSnapshot:
							var snapshot []WsCredit
							if snapBundle, ok := chanData[2].([]interface{}); ok && len(snapBundle) > 0 {
								if _, ok := snapBundle[0].([]interface{}); ok {
									for i := range snapBundle {
										data := snapBundle[i].([]interface{})
										credit := WsCredit{
											ID:           int64(data[0].(float64)),
											Symbol:       data[1].(string),
											Side:         data[2].(string),
											Created:      int64(data[3].(float64)),
											Updated:      int64(data[4].(float64)),
											Amount:       data[5].(float64),
											Flags:        data[6].(string),
											Status:       data[7].(string),
											Rate:         data[11].(float64),
											Period:       int64(data[12].(float64)),
											Opened:       int64(data[13].(float64)),
											LastPayout:   int64(data[14].(float64)),
											Notify:       data[15].(float64) == 1,
											Hidden:       data[16].(float64) == 1,
											Insure:       data[17].(float64) == 1,
											Renew:        data[18].(float64) == 1,
											RateReal:     data[19].(float64),
											NoClose:      data[20].(float64) == 1,
											PositionPair: data[21].(string),
										}
										snapshot = append(snapshot, credit)
									}
									b.Websocket.DataHandler <- snapshot
								}
							}
						case wsFundingCreditNew, wsFundingCreditUpdate, wsFundingCreditCancel:
							if data, ok := chanData[2].([]interface{}); ok && len(data) > 0 {
								b.Websocket.DataHandler <- WsCredit{
									ID:           int64(data[0].(float64)),
									Symbol:       data[1].(string),
									Side:         data[2].(string),
									Created:      int64(data[3].(float64)),
									Updated:      int64(data[4].(float64)),
									Amount:       data[5].(float64),
									Flags:        data[6].(string),
									Status:       data[7].(string),
									Rate:         data[11].(float64),
									Period:       int64(data[12].(float64)),
									Opened:       int64(data[13].(float64)),
									LastPayout:   int64(data[14].(float64)),
									Notify:       data[15].(float64) == 1,
									Hidden:       data[16].(float64) == 1,
									Insure:       data[17].(float64) == 1,
									Renew:        data[18].(float64) == 1,
									RateReal:     data[19].(float64),
									NoClose:      data[20].(float64) == 1,
									PositionPair: data[21].(string),
								}
							}
						case wsFundingLoanSnapshot:
							var snapshot []WsCredit
							if snapBundle, ok := chanData[2].([]interface{}); ok && len(snapBundle) > 0 {
								if _, ok := snapBundle[0].([]interface{}); ok {
									for i := range snapBundle {
										data := snapBundle[i].([]interface{})
										credit := WsCredit{
											ID:         int64(data[0].(float64)),
											Symbol:     data[1].(string),
											Side:       data[2].(string),
											Created:    int64(data[3].(float64)),
											Updated:    int64(data[4].(float64)),
											Amount:     data[5].(float64),
											Flags:      data[6].(string),
											Status:     data[7].(string),
											Rate:       data[11].(float64),
											Period:     int64(data[12].(float64)),
											Opened:     int64(data[13].(float64)),
											LastPayout: int64(data[14].(float64)),
											Notify:     data[15].(float64) == 1,
											Hidden:     data[16].(float64) == 1,
											Insure:     data[17].(float64) == 1,
											Renew:      data[18].(float64) == 1,
											RateReal:   data[19].(float64),
											NoClose:    data[20].(float64) == 1,
										}
										snapshot = append(snapshot, credit)
									}
									b.Websocket.DataHandler <- snapshot
								}
							}
						case wsFundingLoanNew, wsFundingLoanUpdate, wsFundingLoanCancel:
							if data, ok := chanData[2].([]interface{}); ok && len(data) > 0 {
								b.Websocket.DataHandler <- WsCredit{
									ID:         int64(data[0].(float64)),
									Symbol:     data[1].(string),
									Side:       data[2].(string),
									Created:    int64(data[3].(float64)),
									Updated:    int64(data[4].(float64)),
									Amount:     data[5].(float64),
									Flags:      data[6].(string),
									Status:     data[7].(string),
									Rate:       data[11].(float64),
									Period:     int64(data[12].(float64)),
									Opened:     int64(data[13].(float64)),
									LastPayout: int64(data[14].(float64)),
									Notify:     data[15].(float64) == 1,
									Hidden:     data[16].(float64) == 1,
									Insure:     data[17].(float64) == 1,
									Renew:      data[18].(float64) == 1,
									RateReal:   data[19].(float64),
									NoClose:    data[20].(float64) == 1,
								}
							}
						case wsWalletSnapshot:
							var snapshot []WsWallet
							if snapBundle, ok := chanData[2].([]interface{}); ok && len(snapBundle) > 0 {
								if _, ok := snapBundle[0].([]interface{}); ok {
									for i := range snapBundle {
										data := snapBundle[i].([]interface{})
										var balanceAvailable float64
										if _, ok := data[4].(float64); ok {
											balanceAvailable = data[4].(float64)
										}
										wallet := WsWallet{
											Type:              data[0].(string),
											Currency:          data[1].(string),
											Balance:           data[2].(float64),
											UnsettledInterest: data[3].(float64),
											BalanceAvailable:  balanceAvailable,
										}
										snapshot = append(snapshot, wallet)
									}
									b.Websocket.DataHandler <- snapshot
								}
							}
						case wsWalletUpdate:
							if data, ok := chanData[2].([]interface{}); ok && len(data) > 0 {
								var balanceAvailable float64
								if _, ok := data[4].(float64); ok {
									balanceAvailable = data[4].(float64)
								}
								b.Websocket.DataHandler <- WsWallet{
									Type:              data[0].(string),
									Currency:          data[1].(string),
									Balance:           data[2].(float64),
									UnsettledInterest: data[3].(float64),
									BalanceAvailable:  balanceAvailable,
								}
							}
						case wsBalanceUpdate:
							if data, ok := chanData[2].([]interface{}); ok && len(data) > 0 {
								b.Websocket.DataHandler <- WsBalanceInfo{
									TotalAssetsUnderManagement: data[0].(float64),
									NetAssetsUnderManagement:   data[1].(float64),
								}
							}
						case wsMarginInfoUpdate:
							if data, ok := chanData[2].([]interface{}); ok && len(data) > 0 {
								if data[0].(string) == "base" {
									if infoBase, ok := chanData[2].([]interface{}); ok && len(infoBase) > 0 {
										baseData := data[1].([]interface{})
										b.Websocket.DataHandler <- WsMarginInfoBase{
											UserProfitLoss: baseData[0].(float64),
											UserSwaps:      baseData[1].(float64),
											MarginBalance:  baseData[2].(float64),
											MarginNet:      baseData[3].(float64),
										}
									}
								}
							}
						case wsFundingInfoUpdate:
							if data, ok := chanData[2].([]interface{}); ok && len(data) > 0 {
								if data[0].(string) == "sym" {
									symbolData := data[1].([]interface{})
									b.Websocket.DataHandler <- WsFundingInfo{
										YieldLoan:    symbolData[0].(float64),
										YieldLend:    symbolData[1].(float64),
										DurationLoan: symbolData[2].(float64),
										DurationLend: symbolData[3].(float64),
									}
								}
							}
						case wsFundingTradeExecuted, wsFundingTradeUpdate:
							if data, ok := chanData[2].([]interface{}); ok && len(data) > 0 {
								b.Websocket.DataHandler <- WsFundingTrade{
									ID:         int64(data[0].(float64)),
									Symbol:     data[1].(string),
									MTSCreated: int64(data[2].(float64)),
									OfferID:    int64(data[3].(float64)),
									Amount:     data[4].(float64),
									Rate:       data[5].(float64),
									Period:     int64(data[6].(float64)),
									Maker:      data[7].(float64) == 1,
								}
							}
						}
					}
				}
			}
		}
	}
}

// WsInsertSnapshot add the initial orderbook snapshot when subscribed to a
// channel
func (b *Bitfinex) WsInsertSnapshot(p currency.Pair, assetType asset.Item, books []WebsocketBook) error {
	if len(books) == 0 {
		return errors.New("bitfinex.go error - no orderbooks submitted")
	}
	var bid, ask []orderbook.Item
	for i := range books {
		if books[i].Amount > 0 {
			bid = append(bid, orderbook.Item{
				ID:     books[i].ID,
				Amount: books[i].Amount,
				Price:  books[i].Price})
		} else {
			ask = append(ask, orderbook.Item{
				ID:     books[i].ID,
				Amount: books[i].Amount * -1,
				Price:  books[i].Price})
		}
	}

	var newOrderBook orderbook.Base
	newOrderBook.Asks = ask
	newOrderBook.AssetType = assetType
	newOrderBook.Bids = bid
	newOrderBook.Pair = p
	newOrderBook.ExchangeName = b.Name

	err := b.Websocket.Orderbook.LoadSnapshot(&newOrderBook)
	if err != nil {
		return fmt.Errorf("bitfinex.go error - %s", err)
	}
	b.Websocket.DataHandler <- wshandler.WebsocketOrderbookUpdate{Pair: p,
		Asset:    assetType,
		Exchange: b.Name}
	return nil
}

// WsUpdateOrderbook updates the orderbook list, removing and adding to the
// orderbook sides
func (b *Bitfinex) WsUpdateOrderbook(p currency.Pair, assetType asset.Item, book []WebsocketBook) error {
	orderbookUpdate := wsorderbook.WebsocketOrderbookUpdate{
		Asset: assetType,
		Pair:  p,
	}

	for i := range book {
		switch {
		case book[i].Price > 0:
			orderbookUpdate.Action = "update/insert"
			if book[i].Amount > 0 {
				// update bid
				orderbookUpdate.Bids = append(orderbookUpdate.Bids,
					orderbook.Item{
						ID:     book[i].ID,
						Amount: book[i].Amount,
						Price:  book[i].Price})
			} else if book[i].Amount < 0 {
				// update ask
				orderbookUpdate.Asks = append(orderbookUpdate.Asks,
					orderbook.Item{
						ID:     book[i].ID,
						Amount: book[i].Amount * -1,
						Price:  book[i].Price})
			}
		case book[i].Price == 0:
			orderbookUpdate.Action = "delete"
			if book[i].Amount == 1 {
				// delete bid
				orderbookUpdate.Bids = append(orderbookUpdate.Bids,
					orderbook.Item{
						ID: book[i].ID})
			} else if book[i].Amount == -1 {
				// delete ask
				orderbookUpdate.Asks = append(orderbookUpdate.Asks,
					orderbook.Item{
						ID: book[i].ID})
			}
		}
	}
	err := b.Websocket.Orderbook.Update(&orderbookUpdate)
	if err != nil {
		return err
	}

	b.Websocket.DataHandler <- wshandler.WebsocketOrderbookUpdate{Pair: p,
		Asset:    assetType,
		Exchange: b.Name}

	return nil
}

// GenerateDefaultSubscriptions Adds default subscriptions to websocket to be handled by ManageSubscriptions()
func (b *Bitfinex) GenerateDefaultSubscriptions() {
	var channels = []string{
		wsBook,
		wsTrades,
		wsTicker,
		wsCandles,
	}
	var subscriptions []wshandler.WebsocketChannelSubscription
	for i := range channels {
		enabledPairs := b.GetEnabledPairs(asset.Spot)
		for j := range enabledPairs {
			if strings.HasPrefix(enabledPairs[j].Base.String(), "f") {
				log.Warnf(log.WebsocketMgr,
					"%v - Websocket does not support funding currency %v, skipping",
					b.Name, enabledPairs[j])
				continue
			}
			b.appendOptionalDelimiter(&enabledPairs[j])
			params := make(map[string]interface{})
			if channels[i] == wsBook {
				params["prec"] = "R0"
				params["len"] = "100"
			}

			subscriptions = append(subscriptions, wshandler.WebsocketChannelSubscription{
				Channel:  channels[i],
				Currency: enabledPairs[j],
				Params:   params,
			})
		}
	}
	b.Websocket.SubscribeToChannels(subscriptions)
}

// Subscribe sends a websocket message to receive data from the channel
func (b *Bitfinex) Subscribe(channelToSubscribe wshandler.WebsocketChannelSubscription) error {
	req := make(map[string]interface{})
	req["event"] = "subscribe"
	req["channel"] = channelToSubscribe.Channel

	if channelToSubscribe.Currency.String() != "" {
		if channelToSubscribe.Channel == wsCandles {
			// TODO: Add ability to select timescale
			req["key"] = fmt.Sprintf("trade:1D:%v",
				b.FormatExchangeCurrency(channelToSubscribe.Currency, asset.Spot).String())
		} else {
			req["symbol"] = b.FormatExchangeCurrency(channelToSubscribe.Currency,
				asset.Spot).String()
		}
	}

	if len(channelToSubscribe.Params) > 0 {
		for k, v := range channelToSubscribe.Params {
			req[k] = v
		}
	}

	return b.WebsocketConn.SendJSONMessage(req)
}

// Unsubscribe sends a websocket message to stop receiving data from the channel
func (b *Bitfinex) Unsubscribe(channelToSubscribe wshandler.WebsocketChannelSubscription) error {
	req := make(map[string]interface{})
	req["event"] = "unsubscribe"
	req["channel"] = channelToSubscribe.Channel

	if len(channelToSubscribe.Params) > 0 {
		for k, v := range channelToSubscribe.Params {
			req[k] = v
		}
	}
	return b.WebsocketConn.SendJSONMessage(req)
}

// WsSendAuth sends a autheticated event payload
func (b *Bitfinex) WsSendAuth() error {
	if !b.GetAuthenticatedAPISupport(exchange.WebsocketAuthentication) {
		return fmt.Errorf("%v AuthenticatedWebsocketAPISupport not enabled", b.Name)
	}
	nonce := strconv.FormatInt(time.Now().Unix(), 10)
	payload := "AUTH" + nonce
	request := WsAuthRequest{
		Event:       "auth",
		APIKey:      b.API.Credentials.Key,
		AuthPayload: payload,
		AuthSig: crypto.HexEncodeToString(
			crypto.GetHMAC(
				crypto.HashSHA512_384,
				[]byte(payload),
				[]byte(b.API.Credentials.Secret))),
		AuthNonce:     nonce,
		DeadManSwitch: 0,
	}
	err := b.AuthenticatedWebsocketConn.SendJSONMessage(request)
	if err != nil {
		b.Websocket.SetCanUseAuthenticatedEndpoints(false)
		return err
	}
	return nil
}

// WsAddSubscriptionChannel adds a new subscription channel to the
// WebsocketSubdChannels map in bitfinex.go (Bitfinex struct)
func (b *Bitfinex) WsAddSubscriptionChannel(chanID int, channel, pair string) {
	chanInfo := WebsocketChanInfo{Pair: pair, Channel: channel}
	b.WebsocketSubdChannels[chanID] = chanInfo

	if b.Verbose {
		log.Debugf(log.ExchangeSys, "%s Subscribed to Channel: %s Pair: %s ChannelID: %d\n",
			b.Name,
			channel,
			pair,
			chanID)
	}
}

// WsNewOrder authenticated new order request
func (b *Bitfinex) WsNewOrder(data *WsNewOrderRequest) (string, error) {
	data.CustomID = b.AuthenticatedWebsocketConn.GenerateMessageID(false)
	request := makeRequestInterface(wsOrderNew, data)
	resp, err := b.AuthenticatedWebsocketConn.SendMessageReturnResponse(data.CustomID, request)
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", errors.New(b.Name + " - Order message not returned")
	}
	var respData []interface{}
	err = json.Unmarshal(resp, &respData)
	if err != nil {
		return "", err
	}
	responseDataDetail := respData[2].([]interface{})
	responseOrderDetail := responseDataDetail[4].([]interface{})
	var orderID string
	if responseOrderDetail[0] != nil && responseOrderDetail[0].(float64) > 0 {
		orderID = strconv.FormatFloat(responseOrderDetail[0].(float64), 'f', -1, 64)
	}
	errCode := responseDataDetail[6].(string)
	errorMessage := responseDataDetail[7].(string)

	if strings.EqualFold(errCode, wsError) {
		return orderID, errors.New(b.Name + " - " + errCode + ": " + errorMessage)
	}

	return orderID, nil
}

// WsModifyOrder authenticated modify order request
func (b *Bitfinex) WsModifyOrder(data *WsUpdateOrderRequest) error {
	request := makeRequestInterface(wsOrderUpdate, data)
	resp, err := b.AuthenticatedWebsocketConn.SendMessageReturnResponse(data.OrderID, request)
	if err != nil {
		return err
	}
	if resp == nil {
		return errors.New(b.Name + " - Order message not returned")
	}

	var responseData []interface{}
	err = json.Unmarshal(resp, &responseData)
	if err != nil {
		return err
	}
	responseOrderData := responseData[2].([]interface{})
	errCode := responseOrderData[6].(string)
	errorMessage := responseOrderData[7].(string)
	if strings.EqualFold(errCode, wsError) {
		return errors.New(b.Name + " - " + errCode + ": " + errorMessage)
	}

	return nil
}

// WsCancelMultiOrders authenticated cancel multi order request
func (b *Bitfinex) WsCancelMultiOrders(orderIDs []int64) error {
	cancel := WsCancelGroupOrdersRequest{
		OrderID: orderIDs,
	}
	request := makeRequestInterface(wsCancelMultipleOrders, cancel)
	return b.AuthenticatedWebsocketConn.SendJSONMessage(request)
}

// WsCancelOrder authenticated cancel order request
func (b *Bitfinex) WsCancelOrder(orderID int64) error {
	cancel := WsCancelOrderRequest{
		OrderID: orderID,
	}
	request := makeRequestInterface(wsOrderCancel, cancel)
	resp, err := b.AuthenticatedWebsocketConn.SendMessageReturnResponse(orderID, request)
	if err != nil {
		return err
	}
	if resp == nil {
		return fmt.Errorf("%v - Order %v failed to cancel", b.Name, orderID)
	}
	var responseData []interface{}
	err = json.Unmarshal(resp, &responseData)
	if err != nil {
		return err
	}
	responseOrderData := responseData[2].([]interface{})
	errCode := responseOrderData[6].(string)
	errorMessage := responseOrderData[7].(string)
	if strings.EqualFold(errCode, wsError) {
		return errors.New(b.Name + " - " + errCode + ": " + errorMessage)
	}

	return nil
}

// WsCancelAllOrders authenticated cancel all orders request
func (b *Bitfinex) WsCancelAllOrders() error {
	cancelAll := WsCancelAllOrdersRequest{All: 1}
	request := makeRequestInterface(wsCancelMultipleOrders, cancelAll)
	return b.AuthenticatedWebsocketConn.SendJSONMessage(request)
}

// WsNewOffer authenticated new offer request
func (b *Bitfinex) WsNewOffer(data *WsNewOfferRequest) error {
	request := makeRequestInterface(wsFundingOrderNew, data)
	return b.AuthenticatedWebsocketConn.SendJSONMessage(request)
}

// WsCancelOffer authenticated cancel offer request
func (b *Bitfinex) WsCancelOffer(orderID int64) error {
	cancel := WsCancelOrderRequest{
		OrderID: orderID,
	}
	request := makeRequestInterface(wsFundingOrderCancel, cancel)
	resp, err := b.AuthenticatedWebsocketConn.SendMessageReturnResponse(orderID, request)
	if err != nil {
		return err
	}
	if resp == nil {
		return fmt.Errorf("%v - Order %v failed to cancel", b.Name, orderID)
	}
	var responseData []interface{}
	err = json.Unmarshal(resp, &responseData)
	if err != nil {
		return err
	}
	responseOrderData := responseData[2].([]interface{})
	errCode := responseOrderData[6].(string)
	var errorMessage string
	if responseOrderData[7] != nil {
		errorMessage = responseOrderData[7].(string)
	}
	if strings.EqualFold(errCode, wsError) {
		return errors.New(b.Name + " - " + errCode + ": " + errorMessage)
	}

	return nil
}

func makeRequestInterface(channelName string, data interface{}) []interface{} {
	return []interface{}{0, channelName, nil, data}
}
