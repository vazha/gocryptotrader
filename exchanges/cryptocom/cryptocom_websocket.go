package cryptocom

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/thrasher-corp/gocryptotrader/common/crypto"
	"github.com/thrasher-corp/gocryptotrader/currency"
	exchange "github.com/thrasher-corp/gocryptotrader/exchanges"
	"github.com/thrasher-corp/gocryptotrader/exchanges/asset"
	"github.com/thrasher-corp/gocryptotrader/exchanges/order"
	"github.com/thrasher-corp/gocryptotrader/exchanges/orderbook"
	"github.com/thrasher-corp/gocryptotrader/exchanges/stream"
	"github.com/thrasher-corp/gocryptotrader/exchanges/trade"
	"github.com/thrasher-corp/gocryptotrader/log"
)

const (
	cryptocomWebsocket      = "wss://stream.crypto.com/v2"
	btseWebsocketTimer = time.Second * 57
)

// WsConnect connects the websocket client
func (c *Cryptocom) WsConnect() error {
	if !c.Websocket.IsEnabled() || !c.IsEnabled() {
		return errors.New(stream.WebsocketNotEnabled)
	}
	var dialer websocket.Dialer
	err := c.Websocket.Conn.Dial(&dialer, http.Header{})
	if err != nil {
		return err
	}
	c.Websocket.Conn.SetupPingHandler(stream.PingHandler{
		MessageType: websocket.PingMessage,
		Delay:       btseWebsocketTimer,
	})

	go c.wsReadData()
	if c.GetAuthenticatedAPISupport(exchange.WebsocketAuthentication) {
		err = c.WsAuthenticate()
		if err != nil {
			c.Websocket.DataHandler <- err
			c.Websocket.SetCanUseAuthenticatedEndpoints(false)
		}
	}

	return nil
}

// WsAuthenticate Send an authentication message to receive auth data
func (c *Cryptocom) WsAuthenticate() error {
	nonce := strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10)
	path := "/spotWS" + nonce
	hmac := crypto.GetHMAC(crypto.HashSHA512_384,
		[]byte((path)),
		[]byte(c.API.Credentials.Secret),
	)
	sign := crypto.HexEncodeToString(hmac)
	params := Params{
		Channels: []string{c.API.Credentials.Key, nonce, sign},
	}
	req := wsSub{
		Method: "authKeyExpires",
		Params : params,
	}
	return c.Websocket.Conn.SendJSONMessage(req)
}

func stringToOrderStatus(status string) (order.Status, error) {
	switch status {
	case "ORDER_INSERTED", "TRIGGER_INSERTED":
		return order.New, nil
	case "ORDER_CANCELLED":
		return order.Cancelled, nil
	case "ORDER_FULL_TRANSACTED":
		return order.Filled, nil
	case "ORDER_PARTIALLY_TRANSACTED":
		return order.PartiallyFilled, nil
	case "TRIGGER_ACTIVATED":
		return order.Active, nil
	case "INSUFFICIENT_BALANCE":
		return order.InsufficientBalance, nil
	case "MARKET_UNAVAILABLE":
		return order.MarketUnavailable, nil
	default:
		return order.UnknownStatus, errors.New(status + " not recognised as order status")
	}
}

// wsReadData receives and passes on websocket messages for processing
func (c *Cryptocom) wsReadData() {
	c.Websocket.Wg.Add(1)
	defer c.Websocket.Wg.Done()

	for {
		resp := c.Websocket.Conn.ReadMessage()
		if resp.Raw == nil {
			return
		}
		err := c.wsHandleData(resp.Raw)
		if err != nil {
			c.Websocket.DataHandler <- err
		}
	}
}

func (c *Cryptocom) wsHandleData(respRaw []byte) error {
	type Result map[string]interface{}
	var result Result
	err := json.Unmarshal(respRaw, &result)
	if err != nil {
		if strings.Contains(string(respRaw), "connect success") {
			return nil
		}
		return err
	}

	//fmt.Println("WS:", string(respRaw))
	if result == nil {
		return nil
	}

	if result["event"] != nil {
		event, ok := result["event"].(string)
		if !ok {
			return errors.New(c.Name + stream.UnhandledMessage + string(respRaw))
		}
		switch event {
		case "subscribe":
			var subscribe WsSubscriptionAcknowledgement
			err = json.Unmarshal(respRaw, &subscribe)
			if err != nil {
				return err
			}
			log.Infof(log.WebsocketMgr, "%v subscribed to %v", c.Name, strings.Join(subscribe.Channel, ", "))
		case "login":
			var login WsLoginAcknowledgement
			err = json.Unmarshal(respRaw, &login)
			if err != nil {
				return err
			}
			c.Websocket.SetCanUseAuthenticatedEndpoints(login.Success)
			log.Infof(log.WebsocketMgr, "%v websocket authenticated: %v", c.Name, login.Success)
		default:
			return errors.New(c.Name + stream.UnhandledMessage + string(respRaw))
		}
		return nil
	}

	topic, ok := result["result"]
	if !ok {
		return errors.New(c.Name + stream.UnhandledMessage + string(respRaw))
	}
	fmt.Println("WS:", topic)

	switch {
	case topic == "notificationApi":
		var notification wsNotification
		err = json.Unmarshal(respRaw, &notification)
		if err != nil {
			return err
		}
		for i := range notification.Data {
			var oType order.Type
			var oSide order.Side
			var oStatus order.Status
			oType, err = order.StringToOrderType(notification.Data[i].Type)
			if err != nil {
				c.Websocket.DataHandler <- order.ClassificationError{
					Exchange: c.Name,
					OrderID:  notification.Data[i].OrderID,
					Err:      err,
				}
			}
			oSide, err = order.StringToOrderSide(notification.Data[i].OrderMode)
			if err != nil {
				c.Websocket.DataHandler <- order.ClassificationError{
					Exchange: c.Name,
					OrderID:  notification.Data[i].OrderID,
					Err:      err,
				}
			}
			oStatus, err = stringToOrderStatus(notification.Data[i].Status)
			if err != nil {
				c.Websocket.DataHandler <- order.ClassificationError{
					Exchange: c.Name,
					OrderID:  notification.Data[i].OrderID,
					Err:      err,
				}
			}

			var p currency.Pair
			p, err = currency.NewPairFromString(notification.Data[i].Symbol)
			if err != nil {
				return err
			}

			var a asset.Item
			a, err = c.GetPairAssetType(p)
			if err != nil {
				return err
			}

			c.Websocket.DataHandler <- &order.Detail{
				Price:        notification.Data[i].Price,
				Amount:       notification.Data[i].Size,
				TriggerPrice: notification.Data[i].TriggerPrice,
				Exchange:     c.Name,
				ID:           notification.Data[i].OrderID,
				Type:         oType,
				Side:         oSide,
				Status:       oStatus,
				AssetType:    a,
				Date:         time.Unix(0, notification.Data[i].Timestamp*int64(time.Millisecond)),
				Pair:         p,
			}
		}
	case strings.Contains(topic, "tradeHistory"):
		if !c.IsSaveTradeDataEnabled() {
			return nil
		}
		var tradeHistory wsTradeHistory
		err = json.Unmarshal(respRaw, &tradeHistory)
		if err != nil {
			return err
		}
		var trades []trade.Data
		for x := range tradeHistory.Data {
			side := order.Buy
			if tradeHistory.Data[x].Gain == -1 {
				side = order.Sell
			}

			var p currency.Pair
			p, err = currency.NewPairFromString(strings.Replace(tradeHistory.Topic,
				"tradeHistory:",
				"",
				1))
			if err != nil {
				return err
			}
			var a asset.Item
			a, err = c.GetPairAssetType(p)
			if err != nil {
				return err
			}
			trades = append(trades, trade.Data{
				Timestamp:    time.Unix(0, tradeHistory.Data[x].TransactionTime*int64(time.Millisecond)),
				CurrencyPair: p,
				AssetType:    a,
				Exchange:     c.Name,
				Price:        tradeHistory.Data[x].Price,
				Amount:       tradeHistory.Data[x].Amount,
				Side:         side,
				TID:          strconv.FormatInt(tradeHistory.Data[x].ID, 10),
			})
		}
		return trade.AddTradesToBuffer(c.Name, trades...)
	case strings.Contains(topic, "book."):
		var t wsOrderBook
		err = json.Unmarshal(respRaw, &t)
		if err != nil {
			return err
		}
		var newOB orderbook.Base
		var price, amount float64
		for i := range t.Data.SellQuote {
			p := strings.Replace(t.Data.SellQuote[i].Price, ",", "", -1)
			price, err = strconv.ParseFloat(p, 64)
			if err != nil {
				return err
			}
			a := strings.Replace(t.Data.SellQuote[i].Size, ",", "", -1)
			amount, err = strconv.ParseFloat(a, 64)
			if err != nil {
				return err
			}
			if c.orderbookFilter(price, amount) {
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
				return err
			}
			a := strings.Replace(t.Data.BuyQuote[j].Size, ",", "", -1)
			amount, err = strconv.ParseFloat(a, 64)
			if err != nil {
				return err
			}
			if c.orderbookFilter(price, amount) {
				continue
			}
			newOB.Bids = append(newOB.Bids, orderbook.Item{
				Price:  price,
				Amount: amount,
			})
		}
		p, err := currency.NewPairFromString(t.Topic[strings.Index(t.Topic, ":")+1 : strings.Index(t.Topic, currency.UnderscoreDelimiter)])
		if err != nil {
			return err
		}
		var a asset.Item
		a, err = c.GetPairAssetType(p)
		if err != nil {
			return err
		}
		newOB.Pair = p
		newOB.AssetType = a
		newOB.ExchangeName = c.Name
		orderbook.Reverse(newOB.Asks) // Reverse asks for correct alignment
		newOB.VerificationBypass = c.OrderbookVerificationBypass
		err = c.Websocket.Orderbook.LoadSnapshot(&newOB)
		if err != nil {
			return err
		}
	default:
		return errors.New(c.Name + stream.UnhandledMessage + string(respRaw))
	}

	return nil
}

// orderbookFilter is needed on book levels from this exchange as their data
// is incorrect
func (c *Cryptocom) orderbookFilter(price, amount float64) bool {
	// Amount filtering occurs when the amount exceeds the decimal returned.
	// e.g. {"price":"1.37","size":"0.00"} currency: SFI-ETH
	// Opted to not round up to 0.01 as this might skew calculations
	// more than removing from the books completely.

	// Price filtering occurs when we are deep in the bid book and there are
	// prices that are less than 4 decimal places
	// e.g. {"price":"0.0000","size":"14219"} currency: TRX-PAX
	// We cannot load a zero price and this will ruin calculations
	return price == 0 || amount == 0
}

// GenerateDefaultSubscriptions Adds default subscriptions to websocket to be handled by ManageSubscriptions()
func (c *Cryptocom) GenerateDefaultSubscriptions() ([]stream.ChannelSubscription, error) {
	var channels = []string{"book.%s.150", "trade.%s"}
	pairs, err := c.GetEnabledPairs(asset.Spot)
	if err != nil {
		return nil, err
	}
	var subscriptions []stream.ChannelSubscription
	if c.Websocket.CanUseAuthenticatedEndpoints() {
		subscriptions = append(subscriptions, stream.ChannelSubscription{
			Channel: "notificationApi",
		})
	}
	for i := range channels {
		for j := range pairs {
			subscriptions = append(subscriptions, stream.ChannelSubscription{
				Channel:  fmt.Sprintf(channels[i], pairs[j]),
				Currency: pairs[j],
				Asset:    asset.Spot,
			})
		}
	}
	return subscriptions, nil
}

// Subscribe sends a websocket message to receive data from the channel
func (c *Cryptocom) Subscribe(channelsToSubscribe []stream.ChannelSubscription) error {
	var sub wsSub
	sub.Method = "subscribe"
	for i := range channelsToSubscribe {
		sub.Params.Channels = append(sub.Params.Channels, channelsToSubscribe[i].Channel)
	}

	sub.Nonce = time.Now().Unix()

	err := c.Websocket.Conn.SendJSONMessage(sub)
	if err != nil {
		return err
	}
	c.Websocket.AddSuccessfulSubscriptions(channelsToSubscribe...)
	return nil
}

// Unsubscribe sends a websocket message to stop receiving data from the channel
func (c *Cryptocom) Unsubscribe(channelsToUnsubscribe []stream.ChannelSubscription) error {
	var unSub wsSub
	unSub.Method = "unsubscribe"
	for i := range channelsToUnsubscribe {
		unSub.Params.Channels = append(unSub.Params.Channels,
			channelsToUnsubscribe[i].Channel)
	}
	err := c.Websocket.Conn.SendJSONMessage(unSub)
	if err != nil {
		return err
	}
	c.Websocket.RemoveSuccessfulUnsubscriptions(channelsToUnsubscribe...)
	return nil
}
