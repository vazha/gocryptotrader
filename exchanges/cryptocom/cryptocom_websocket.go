package cryptocom

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/vazha/gocryptotrader/common"
	"github.com/vazha/gocryptotrader/exchanges/ticker"
	"github.com/vazha/gocryptotrader/log"
	"net/http"
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
	"github.com/vazha/gocryptotrader/exchanges/stream"
	"github.com/vazha/gocryptotrader/exchanges/trade"
)

const (
	cryptocomWebsocket      = "wss://stream.crypto.com/v2/market"
	cryptocomAuthWebsocket  = "wss://stream.crypto.com/v2/user" // "wss://uat-stream.3ona.co/v2/user"
	cryptocomWebsocketTimer = time.Second * 57
)

var authenticatedChannels = []string{"user.balance", "user.order"}

// WsConnect connects the websocket client
func (c *Cryptocom) WsConnect() error {
	if !c.Websocket.IsEnabled() || !c.IsEnabled() {
		return errors.New(stream.WebsocketNotEnabled)
	}
	var dialer websocket.Dialer
	//fmt.Printf("websocket.Dialer: %+v\n", dialer)
	dialer.HandshakeTimeout = 5000000000
	err := c.Websocket.Conn.Dial(&dialer, http.Header{})
	if err != nil {
		return err
	}

	comms := make(chan stream.Response)
	go c.wsReadData(comms)
	go c.wsFunnelConnectionData(c.Websocket.Conn, comms)
	if c.GetAuthenticatedAPISupport(exchange.WebsocketAuthentication) {
		err = c.Websocket.AuthConn.Dial(&dialer, http.Header{})
		if err != nil {
			c.Websocket.SetCanUseAuthenticatedEndpoints(false)
			log.Errorf(log.ExchangeSys,
				"%v - failed to connect to authenticated endpoint: %v\n",
				c.Name,
				err)
		} else {
			go c.wsFunnelConnectionData(c.Websocket.AuthConn, comms)
			err = c.WsAuthenticate()
			if err != nil {
				fmt.Println("Authen fail:", err)
				c.Websocket.DataHandler <- err
				c.Websocket.SetCanUseAuthenticatedEndpoints(false)
			}
			fmt.Println("Authen request sent")
		}
	}

	//time.Sleep(time.Second)
	return nil
}

// wsFunnelConnectionData funnels both auth and public ws data into one manageable place
func (c *Cryptocom) wsFunnelConnectionData(ws stream.Connection, comms chan stream.Response) {
	//c.Websocket.Wg.Add(1)
	//defer c.Websocket.Wg.Done()
	defer func() {
		fmt.Printf("%s, wsFunnelConnectionData exit, wg: %v\n", c.Name, c.Websocket.Wg)
	}()

	for {
		resp := ws.ReadMessage()
		if resp.Raw == nil {
			return
		}
		comms <- resp
	}
}

// WsAuthenticate Send an authentication message to receive auth data
func (c *Cryptocom) WsAuthenticate() error {
	nonce := time.Now().UnixNano()/int64(time.Millisecond)
	//time.Sleep(time.Second * 2)
	endpoint := cryptocomWsAuth
	var id int64 = 0
	hmac := crypto.GetHMAC(
		crypto.HashSHA256,
		[]byte(endpoint + strconv.Itoa(int(id)) + c.API.Credentials.Key + fmt.Sprint(nonce)),
		[]byte(c.API.Credentials.Secret),
	)

	r := wsAuth{
		ID: id,
		Method: endpoint,
		ApiKey: c.API.Credentials.Key,
		Sig: crypto.HexEncodeToString(hmac),
		Nonce: nonce,
	}

	return c.Websocket.AuthConn.SendJSONMessage(r)
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
func (c *Cryptocom) wsReadData(comms chan stream.Response) {
	c.Websocket.Wg.Add(1)
	defer c.Websocket.Wg.Done()

	for {
		select {
		case <-c.Websocket.ShutdownC:
			return
		case resp := <-comms:
			go func() {
				err := c.wsHandleData(resp)
				if err != nil {
					c.Websocket.DataHandler <- fmt.Errorf("%s - unhandled websocket data: %v",
						c.Name,
						err)
				}
			}()
		}
	}
}

func (c *Cryptocom) wsHandleData(resp stream.Response) error {
	respRaw := resp.Raw
	var result WsSubRead
	err := json.Unmarshal(respRaw, &result)
	if err != nil {
		return err
	}

	//fmt.Println("WS:", string(respRaw))
	//if result == nil {
	//	return nil
	//}

	//if result.Method != "" {
	//	switch result.Method {
	//	case "subscribe":
	//		var subscribe WsSubscriptionAcknowledgement
	//		err = json.Unmarshal(respRaw, &subscribe)
	//		if err != nil {
	//			return err
	//		}
	//		log.Infof(log.WebsocketMgr, "%v subscribed to %v", c.Name, strings.Join(subscribe.Channel, ", "))
	//	case "login":
	//		var login WsLoginAcknowledgement
	//		err = json.Unmarshal(respRaw, &login)
	//		if err != nil {
	//			return err
	//		}
	//		c.Websocket.SetCanUseAuthenticatedEndpoints(login.Success)
	//		log.Infof(log.WebsocketMgr, "%v websocket authenticated: %v", c.Name, login.Success)
	//	default:
	//		return errors.New(c.Name + stream.UnhandledMessage + string(respRaw))
	//	}
	//	return nil
	//}

	//topic, ok := result["result"]
	//if !ok {
	//	return errors.New(c.Name + stream.UnhandledMessage + string(respRaw))
	//}
	// fmt.Println("WS:", result)
	// fmt.Printf("WSS: %+v\n", result)

	switch {
	case result.Method == "public/auth":
	fmt.Println("public/auth ACCEPTED")
	case result.Method == "subscribe" && result.Result.Channel == "":
		return nil
	case result.Method == "public/heartbeat":
		return c.SendHeartbeat(result.ID, resp.Auth)
	case result.Result.Channel == "user.balance":
		// fmt.Println("user.balance:", result.Result.Data)
		var balanceUpdate Bals
		for i := range result.Result.Data {
			if data, ok := result.Result.Data[i].(map[string]interface{}); ok {
				for k, v := range data {
					switch k {
					case "balance":
						balanceUpdate.Balance = v.(float64)
					case "available":
						balanceUpdate.Available = v.(float64)
					case"order":
						balanceUpdate.Order = v.(float64)
					case"stake":
						balanceUpdate.Stake = v.(float64)
					case "currency":
						balanceUpdate.Currency = v.(string)
					}
				}
			}
		}

		c.Websocket.DataHandler <- balanceUpdate
	case result.Result.Channel == "user.order":
		// fmt.Println("user.order:", result.Result.Data)
		//orders := make(map[string]UserOrderResponse)
		for i := range result.Result.Data {
			if data, ok := result.Result.Data[i].(map[string]interface{}); ok {
				var orderID, orderStatus string
				var orderReason float64
				for k, v := range data {
					switch k {
					case "status":
						orderStatus = v.(string)
					case "reason":
						orderReason = v.(float64)
					case"order_id":
						orderID = v.(string)
					}
				}

				sent := LocalMatcher.IncomingWithData(orderID, UserOrderResponse{
					OrderId: orderID,
					Status: orderStatus,
					Reason: orderReason,
				})

				if !sent {
					//fmt.Printf("Local matcher IncomingWithData not sent for %s\n", orderID)
				}

			}
		}
	case strings.Contains(result.Result.Channel, "trade"):
		if !c.IsSaveTradeDataEnabled() {
			return nil
		}
		var trades []trade.Data
		for x := range result.Result.Data {
			t := result.Result.Data[x].(wsTradeData)
			side := order.Buy
			if t.Side == order.Sell.String() {
				side = order.Sell
			}

			var p currency.Pair
			p, err = currency.NewPairFromString(result.Result.InstrumentName)
			if err != nil {
				return err
			}
			var a asset.Item
			a, err = c.GetPairAssetType(p)
			if err != nil {
				return err
			}
			trades = append(trades, trade.Data{
				Timestamp:    time.Unix(0, t.CreateTime),
				CurrencyPair: p,
				AssetType:    a,
				Exchange:     c.Name,
				Price:        t.TradedPrice,
				Amount:       t.TradedQuantity,
				Side:         side,
				TID:          t.OrderId,
			})
		}
		return trade.AddTradesToBuffer(c.Name, trades...)
	case strings.Contains(result.Result.Channel, "ticker"):
		var t WsReadTicker
		err := json.Unmarshal(respRaw, &t)
		if err != nil {
			return fmt.Errorf("%v - Could not convert to a TickerStream structure %s",
				c.Name,
				err.Error())
		}
		pairs, err := c.GetEnabledPairs(asset.Spot)
		if err != nil {
			return err
		}

		format, err := c.GetPairFormat(asset.Spot, true)
		if err != nil {
			return err
		}

		pair, err := currency.NewPairFromFormattedPairs(t.Result.InstrumentName, pairs, format)
		if err != nil {
			return err
		}

		time := time.Unix(t.Result.Data[0].T / 1000, 0)

		//fmt.Println("WS Ticker:", time, pair)

		c.Websocket.DataHandler <- &ticker.Price{
			ExchangeName: c.Name,
			//Open:         t.OpenPrice,
			//Close:        t.ClosePrice,
			Volume:       t.Result.Data[0].V,
			//QuoteVolume:  t.TotalTradedQuoteVolume,
			High:         t.Result.Data[0].H,
			Low:          t.Result.Data[0].L,
			Bid:          t.Result.Data[0].B,
			Ask:          t.Result.Data[0].K,
			Last:         t.Result.Data[0].A,
			LastUpdated:  time,
			AssetType:    asset.Spot,
			Pair:         pair,
		}
	case strings.Contains(result.Result.Channel, "book"):
		var ob WsReadOrderBook
		err = json.Unmarshal(respRaw, &ob)
		if err != nil {
			return err
		}

		var newOB orderbook.Base

		for i := range ob.Result.Data[0].Asks {
			if c.orderbookFilter(ob.Result.Data[0].Asks[i][0], ob.Result.Data[0].Asks[i][1]) {
				continue
			}
			newOB.Asks = append(newOB.Asks, orderbook.Item{
				Price:  ob.Result.Data[0].Asks[i][0],
				Amount: ob.Result.Data[0].Asks[i][1],
			})
		}
		for i := range ob.Result.Data[0].Bids {
			if c.orderbookFilter(ob.Result.Data[0].Bids[i][0], ob.Result.Data[0].Bids[i][1]) {
				continue
			}
			newOB.Bids = append(newOB.Bids, orderbook.Item{
				Price:  ob.Result.Data[0].Bids[i][0],
				Amount: ob.Result.Data[0].Bids[i][1],
			})
		}

		p, err := currency.NewPairFromString(result.Result.InstrumentName)
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
	var channels = []string{"book.%s.150", "trade.%s", "ticker.%s"}
	//var channels = []string{"book.%s.150"}
	pairs, err := c.GetEnabledPairs(asset.Spot)
	if err != nil {
		return nil, err
	}
	var subscriptions []stream.ChannelSubscription
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

// GenerateAuthenticatedSubscriptions Adds default subscriptions to auth websocket
func (c *Cryptocom) GenerateAuthenticatedSubscriptions() ([]stream.ChannelSubscription, error) {
	var subscriptions []stream.ChannelSubscription
	for i := range authenticatedChannels {
		subscriptions = append(subscriptions, stream.ChannelSubscription{
			Channel: authenticatedChannels[i],
		})
	}
	return subscriptions, nil
}

// Subscribe sends a websocket message to receive data from the channel
func (c *Cryptocom) Subscribe(channelsToSubscribe []stream.ChannelSubscription) error {
	var sub wsSub
	sub.Method = "subscribe"
	var ch, authCh []string
	for i := range channelsToSubscribe {
		//sub.Params.Channels = append(sub.Params.Channels, channelsToSubscribe[i].Channel)
		if common.StringDataContains(authenticatedChannels, channelsToSubscribe[i].Channel) {
			authCh = append(authCh, channelsToSubscribe[i].Channel) // authenticated channels
		}else{
			ch = append(ch, channelsToSubscribe[i].Channel)
		}
	}

	if len(authCh) > 0 {
		sub.Params = map[string]interface{}{
			"channels": authCh,
		}
		sub.Nonce = time.Now().Unix()

		err := c.Websocket.AuthConn.SendJSONMessage(sub)
		if err != nil {
			return err
		}
	}

	p := map[string]interface{}{
		"channels": ch,
	}
	sub.Params = p
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

	var ch, authCh []string
	for i := range channelsToUnsubscribe {
		//sub.Params.Channels = append(sub.Params.Channels, channelsToSubscribe[i].Channel)
		if common.StringDataContains(authenticatedChannels, channelsToUnsubscribe[i].Channel) {
			authCh = append(authCh, channelsToUnsubscribe[i].Channel) // authenticated channels
		}else{
			ch = append(ch, channelsToUnsubscribe[i].Channel)
		}
	}

	if len(authCh) > 0 {
		unSub.Nonce = time.Now().Unix()
		unSub.Params = map[string]interface{}{
			"channels": authCh,
		}

		err := c.Websocket.AuthConn.SendJSONMessage(unSub)
		if err != nil {
			return err
		}
	}

	unSub.Nonce = time.Now().Unix()
	unSub.Params = map[string]interface{}{
		"channels": ch,
	}

	err := c.Websocket.Conn.SendJSONMessage(unSub)
	if err != nil {
		return err
	}
	c.Websocket.RemoveSuccessfulUnsubscriptions(channelsToUnsubscribe...)
	return nil
}

// SendHeartbeat sends response to server heartbeat
func (c *Cryptocom) SendHeartbeat(ID int64, auth bool) error {
	var unSub wsSub
	unSub.Method = "public/respond-heartbeat"
	unSub.ID = ID

	var err error
	if auth {
		err = c.Websocket.AuthConn.SendJSONMessage(unSub)
	}else{
		err = c.Websocket.Conn.SendJSONMessage(unSub)
	}

	return err
}