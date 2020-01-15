package btse

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vazha/gocryptotrader/currency"
	"github.com/vazha/gocryptotrader/exchanges/asset"
	"github.com/vazha/gocryptotrader/exchanges/order"
	"github.com/vazha/gocryptotrader/exchanges/orderbook"
	"github.com/vazha/gocryptotrader/exchanges/websocket/wshandler"
	log "github.com/vazha/gocryptotrader/logger"
)

const (
	btseWebsocket      = "wss://ws.btse.com/spotWS"
	btseWebsocketTimer = 57 * time.Second
)

// WsConnect connects the websocket client
func (b *BTSE) WsConnect() error {
	if !b.Websocket.IsEnabled() || !b.IsEnabled() {
		return errors.New(wshandler.WebsocketNotEnabled)
	}
	var dialer websocket.Dialer
	err := b.WebsocketConn.Dial(&dialer, http.Header{})
	if err != nil {
		return err
	}
	b.WebsocketConn.SetupPingHandler(wshandler.WebsocketPingHandler{
		MessageType: websocket.PingMessage,
		Delay:       btseWebsocketTimer,
	})
	go b.WsHandleData()
	b.GenerateDefaultSubscriptions()

	return nil
}

// WsHandleData handles read data from websocket connection
func (b *BTSE) WsHandleData() {
	b.Websocket.Wg.Add(1)

	defer func() {
		b.Websocket.Wg.Done()
	}()

	for {
		select {
		case <-b.Websocket.ShutdownC:
			return

		default:
			resp, err := b.WebsocketConn.ReadMessage()
			if err != nil {
				b.Websocket.ReadMessageErrors <- err
				return
			}
			b.Websocket.TrafficAlert <- struct{}{}

			type Result map[string]interface{}
			result := Result{}
			err = json.Unmarshal(resp.Raw, &result)
			if err != nil {
				b.Websocket.DataHandler <- err
				continue
			}
			switch {
			case strings.Contains(result["topic"].(string), "tradeHistory"):
				var tradeHistory wsTradeHistory
				err = json.Unmarshal(resp.Raw, &tradeHistory)
				if err != nil {
					b.Websocket.DataHandler <- err
					continue
				}
				for x := range tradeHistory.Data {
					side := order.Buy.String()
					if tradeHistory.Data[x].Gain == -1 {
						side = order.Sell.String()
					}
					b.Websocket.DataHandler <- wshandler.TradeData{
						Timestamp:    time.Unix(0, tradeHistory.Data[x].TransactionTime*int64(time.Millisecond)),
						CurrencyPair: currency.NewPairFromString(strings.Replace(tradeHistory.Topic, "tradeHistory:", "", 1)),
						AssetType:    asset.Spot,
						Exchange:     b.Name,
						Price:        tradeHistory.Data[x].Price,
						Amount:       tradeHistory.Data[x].Amount,
						Side:         side,
					}
				}
			case strings.Contains(result["topic"].(string), "orderBookApi"):
				var t wsOrderBook
				err = json.Unmarshal(resp.Raw, &t)
				if err != nil {
					b.Websocket.DataHandler <- err
					continue
				}
				var newOB orderbook.Base
				var price, amount float64
				for i := range t.Data.SellQuote {
					p := strings.Replace(t.Data.SellQuote[i].Price, ",", "", -1)
					price, err = strconv.ParseFloat(p, 64)
					if err != nil {
						b.Websocket.DataHandler <- err
						continue
					}
					a := strings.Replace(t.Data.SellQuote[i].Size, ",", "", -1)
					amount, err = strconv.ParseFloat(a, 64)
					if err != nil {
						b.Websocket.DataHandler <- err
						continue
					}
					newOB.Asks = append(newOB.Asks, orderbook.Item{
						Price:  price,
						Amount: amount,
					})
				}
				for j := range t.Data.BuyQuote {
					p := strings.Replace(t.Data.BuyQuote[j].Price, ",", "", -1)
					price, err = strconv.ParseFloat(p, 64)
					if err != nil {
						b.Websocket.DataHandler <- err
						continue
					}
					a := strings.Replace(t.Data.BuyQuote[j].Size, ",", "", -1)
					amount, err = strconv.ParseFloat(a, 64)
					if err != nil {
						b.Websocket.DataHandler <- err
						continue
					}
					newOB.Bids = append(newOB.Bids, orderbook.Item{
						Price:  price,
						Amount: amount,
					})
				}
				newOB.AssetType = asset.Spot
				newOB.Pair = currency.NewPairFromString(t.Topic[strings.Index(t.Topic, ":")+1 : strings.Index(t.Topic, "_")])
				newOB.ExchangeName = b.Name
				err = b.Websocket.Orderbook.LoadSnapshot(&newOB)
				if err != nil {
					b.Websocket.DataHandler <- err
					continue
				}
				b.Websocket.DataHandler <- wshandler.WebsocketOrderbookUpdate{Pair: newOB.Pair,
					Asset:    asset.Spot,
					Exchange: b.Name}
			default:
				log.Warnf(log.ExchangeSys,
					"%s: unhandled websocket response: %s", b.Name, resp.Raw)
			}
		}
	}
}

// GenerateDefaultSubscriptions Adds default subscriptions to websocket to be handled by ManageSubscriptions()
func (b *BTSE) GenerateDefaultSubscriptions() {
	var channels = []string{"orderBookApi:%s_0", "tradeHistory:%s"}
	pairs := b.GetEnabledPairs(asset.Spot)
	var subscriptions []wshandler.WebsocketChannelSubscription
	for i := range channels {
		for j := range pairs {
			subscriptions = append(subscriptions, wshandler.WebsocketChannelSubscription{
				Channel:  fmt.Sprintf(channels[i], pairs[j]),
				Currency: pairs[j],
			})
		}
	}
	b.Websocket.SubscribeToChannels(subscriptions)
}

// Subscribe sends a websocket message to receive data from the channel
func (b *BTSE) Subscribe(channelToSubscribe wshandler.WebsocketChannelSubscription) error {
	var sub wsSub
	sub.Operation = "subscribe"
	sub.Arguments = []string{channelToSubscribe.Channel}
	return b.WebsocketConn.SendJSONMessage(sub)
}

// Unsubscribe sends a websocket message to stop receiving data from the channel
func (b *BTSE) Unsubscribe(channelToSubscribe wshandler.WebsocketChannelSubscription) error {
	var unSub wsSub
	unSub.Operation = "unsubscribe"
	unSub.Arguments = []string{channelToSubscribe.Channel}
	return b.WebsocketConn.SendJSONMessage(unSub)
}
