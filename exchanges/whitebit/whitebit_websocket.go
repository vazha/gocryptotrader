package whitebit

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/thrasher-corp/gocryptotrader/common"
	"github.com/thrasher-corp/gocryptotrader/currency"
	"github.com/thrasher-corp/gocryptotrader/exchanges/asset"
	"github.com/thrasher-corp/gocryptotrader/exchanges/orderbook"
	"github.com/thrasher-corp/gocryptotrader/exchanges/stream"
	"github.com/thrasher-corp/gocryptotrader/exchanges/stream/buffer"
	"github.com/thrasher-corp/gocryptotrader/log"
)

var comms = make(chan stream.Response)

type checksum struct {
	Token    int
	Sequence int64
}

var authToken string

// checksumStore quick global for now
var checksumStore = make(map[int]*checksum)
var cMtx sync.Mutex

// WsConnect starts a new websocket connection
func (b *Whitebit) WsConnect() error {
	if !b.Websocket.IsEnabled() || !b.IsEnabled() {
		return errors.New(stream.WebsocketNotEnabled)
	}

	var dialer websocket.Dialer
	err := b.Websocket.Conn.Dial(&dialer, http.Header{})
	if err != nil {
		return fmt.Errorf("%v unable to connect to Websocket. Error: %s",
			b.Name,
			err)
	}

	go b.wsReadData(b.Websocket.Conn)

	//if b.GetAuthenticatedAPISupport(exchange.WebsocketAuthentication) {
	if b.Websocket.CanUseAuthenticatedEndpoints() {
		authToken, err = b.GetWebsocketToken()
		if err != nil {
			b.Websocket.SetCanUseAuthenticatedEndpoints(false)
			log.Errorf(log.ExchangeSys,
				"%v - authentication failed: %v\n",
				b.Name,
				err)
		} else {
			err = b.WsSendAuth()
			if err != nil {
				log.Errorf(log.ExchangeSys,
					"%v - authentication failed: %v\n",
					b.Name,
					err)
				b.Websocket.SetCanUseAuthenticatedEndpoints(false)
			}
		}
	}

	go b.WsDataHandler()
	return nil
}

// wsReadData receives and passes on websocket messages for processing
func (b *Whitebit) wsReadData(ws stream.Connection) {
	b.Websocket.Wg.Add(1)
	defer b.Websocket.Wg.Done()
	for {
		resp := ws.ReadMessage()
		if resp.Raw == nil {
			return
		}
		comms <- resp
	}
}

// WsDataHandler handles data from wsReadData
func (b *Whitebit) WsDataHandler() {
	b.Websocket.Wg.Add(1)
	defer b.Websocket.Wg.Done()
	for {
		select {
		case resp := <-comms:
			if resp.Type == websocket.TextMessage {
				err := b.wsHandleData(resp.Raw)
				if err != nil {
					b.Websocket.DataHandler <- err
				}
			}
		case <-b.Websocket.ShutdownC:
			return
		}
	}
}

func (b *Whitebit) wsHandleData(respRaw []byte) error {
	var result WsRequest
	err := json.Unmarshal(respRaw, &result)
	if err != nil {
		return err
	}

	switch result.Method {
	case "depth_update":
		switch result.Params[0] {
		case true: // book snapshot
			var book orderbook.Base
			pair, err := currency.NewPairFromString(result.Params[2].(string))
			if err != nil {
				return err
			}

			data := result.Params[1].(map[string]interface{}) // [][]float64
			for k, v := range data {
				d := v.([]interface{})
				for i := range d {
					f := d[i].([]interface{})
					amount, err := strconv.ParseFloat(f[1].(string), 64)
					if err != nil {
						return err
					}
					price, err := strconv.ParseFloat(f[0].(string), 64)
					if err != nil {
						return err
					}
					item := orderbook.Item{
						ID:     result.ID,
						Amount: amount,
						Price: price,
					}

					if k == "asks" {
						book.Asks = append(book.Asks, item)
					} else if k == "bids" {
						book.Bids = append(book.Bids, item)
					}
				}
			}

			book.AssetType = asset.Spot
			book.Pair = pair
			book.ExchangeName = b.Name
			book.NotAggregated = true
			book.HasChecksumValidation = true
			book.IsFundingRate = true
			book.VerificationBypass = b.OrderbookVerificationBypass

			err = b.Websocket.Orderbook.LoadSnapshot(&book)
			//fmt.Println("BOOK set:", book.Pair, err)
			return err
		case false: // update book
			pair, err := currency.NewPairFromString(result.Params[2].(string))
			if err != nil {
				return err
			}


			var Asks []orderbook.Item
			var Bids []orderbook.Item

			data := result.Params[1].(map[string]interface{}) // [][]float64
			for k, v := range data {
				d := v.([]interface{})
				for i := range d {
					f := d[i].([]interface{})
					amount, err := strconv.ParseFloat(f[1].(string), 64)
					if err != nil {
						return err
					}
					price, err := strconv.ParseFloat(f[0].(string), 64)
					if err != nil {
						return err
					}
					item := orderbook.Item{
						ID:     result.ID,
						Amount: amount,
						Price: price,
					}

					if k == "asks" {
						Asks = append(Asks, item)
					} else if k == "bids" {
						Bids = append(Bids, item)
					}
				}
			}

			err = b.Websocket.Orderbook.Update(&buffer.Update{
				Bids:     Bids,
				Asks:     Asks,
				Pair:     pair,
				UpdateID: result.ID,
				Asset:    asset.Spot,
			})

			//fmt.Println("BOOK upd:", Asks, Bids, err)
			return err
		}
	case "ticker":
		//chanF, ok := d[0].(float64)
		//if !ok {
		//	return errors.New("channel ID type assertion failure")
		//}

		//chanID := int(chanF)
		//var datum string
		//if datum, ok = d[1].(string); ok {
		//	// Capturing heart beat
		//	if datum == "hb" {
		//		return nil
		//	}
		//
		//	// Capturing checksum and storing value
		//	if datum == "cs" {
		//		var tokenF float64
		//		tokenF, ok = d[2].(float64)
		//		if !ok {
		//			return errors.New("checksum token type assertion failure")
		//		}
		//		var seqNoF float64
		//		seqNoF, ok = d[3].(float64)
		//		if !ok {
		//			return errors.New("sequence number type assertion failure")
		//		}
		//
		//		cMtx.Lock()
		//		checksumStore[chanID] = &checksum{
		//			Token:    int(tokenF),
		//			Sequence: int64(seqNoF),
		//		}
		//		cMtx.Unlock()
		//		return nil
		//	}
		//}

		//chanInfo, ok := b.WebsocketSubdChannels[chanID]
		//if !ok && chanID != 0 {
		//	return fmt.Errorf("bitfinex.go error - Unable to locate chanID: %d",
		//		chanID)
		//}

		//var chanAsset = asset.Spot
		//var pair currency.Pair
		//pairInfo := strings.Split(chanInfo.Pair, ":")
		//switch {
		//case len(pairInfo) >= 3:
		//	newPair := pairInfo[2]
		//	if newPair[0] == 'f' {
		//		chanAsset = asset.MarginFunding
		//	}
		//
		//	pair, err = currency.NewPairFromString(newPair[1:])
		//	if err != nil {
		//		return err
		//	}
		//case len(pairInfo) == 1:
		//	newPair := pairInfo[0]
		//	if newPair[0] == 'f' {
		//		chanAsset = asset.MarginFunding
		//	}
		//
		//	pair, err = currency.NewPairFromString(newPair[1:])
		//	if err != nil {
		//		return err
		//	}
		//case chanInfo.Pair != "":
		//	if strings.Contains(chanInfo.Pair, ":") {
		//		chanAsset = asset.Margin
		//	}
		//
		//	pair, err = currency.NewPairFromString(chanInfo.Pair[1:])
		//	if err != nil {
		//		return err
		//	}
		//}

		if false {
			switch "authResp" {
			case wsHeartbeat, pong:
				return nil
			case wsNotification:
				//notification := d[2].([]interface{})
				//if data, ok := notification[4].([]interface{}); ok {
				//	channelName := notification[1].(string)
				//	switch {
				//	case strings.Contains(channelName, wsFundingOrderNewRequest),
				//		strings.Contains(channelName, wsFundingOrderUpdateRequest),
				//		strings.Contains(channelName, wsFundingOrderCancelRequest):
				//		if data[0] != nil && data[0].(float64) > 0 {
				//			id := int64(data[0].(float64))
				//			if b.Websocket.Match.IncomingWithData(id, respRaw) {
				//				return nil
				//			}
				//			b.wsHandleFundingOffer(data)
				//		}
				//	case strings.Contains(channelName, wsOrderNewRequest),
				//		strings.Contains(channelName, wsOrderUpdateRequest),
				//		strings.Contains(channelName, wsOrderCancelRequest):
				//		if data[2] != nil && data[2].(float64) > 0 {
				//			id := int64(data[2].(float64))
				//			if b.Websocket.Match.IncomingWithData(id, respRaw) {
				//				return nil
				//			}
				//			b.wsHandleOrder(data)
				//		}
				//
				//	default:
				//		return fmt.Errorf("%s - Unexpected data returned %s",
				//			b.Name,
				//			respRaw)
				//	}
				//}
				//if notification[5] != nil &&
				//	strings.EqualFold(notification[5].(string), wsError) {
				//	return fmt.Errorf("%s - Error %s",
				//		b.Name,
				//		notification[6].(string))
				//}
			case wsOrderSnapshot:
				//if snapBundle, ok := d[2].([]interface{}); ok && len(snapBundle) > 0 {
				//	if _, ok := snapBundle[0].([]interface{}); ok {
				//		for i := range snapBundle {
				//			positionData := snapBundle[i].([]interface{})
				//			b.wsHandleOrder(positionData)
				//		}
				//	}
				//}
			case wsOrderCancel, wsOrderNew, wsOrderUpdate:
				//if oData, ok := d[2].([]interface{}); ok && len(oData) > 0 {
				//	b.wsHandleOrder(oData)
				//}
			case wsPositionSnapshot:
				//var snapshot []WebsocketPosition
				//if snapBundle, ok := d[2].([]interface{}); ok && len(snapBundle) > 0 {
				//	if _, ok := snapBundle[0].([]interface{}); ok {
				//		for i := range snapBundle {
				//			positionData := snapBundle[i].([]interface{})
				//			position := WebsocketPosition{
				//				Pair:              positionData[0].(string),
				//				Status:            positionData[1].(string),
				//				Amount:            positionData[2].(float64),
				//				Price:             positionData[3].(float64),
				//				MarginFunding:     positionData[4].(float64),
				//				MarginFundingType: int64(positionData[5].(float64)),
				//				ProfitLoss:        positionData[6].(float64),
				//				ProfitLossPercent: positionData[7].(float64),
				//				LiquidationPrice:  positionData[8].(float64),
				//				Leverage:          positionData[9].(float64),
				//			}
				//			snapshot = append(snapshot, position)
				//		}
				//		b.Websocket.DataHandler <- snapshot
				//	}
				//}
			case wsPositionNew, wsPositionUpdate, wsPositionClose:
				//if positionData, ok := d[2].([]interface{}); ok && len(positionData) > 0 {
				//	position := WebsocketPosition{
				//		Pair:              positionData[0].(string),
				//		Status:            positionData[1].(string),
				//		Amount:            positionData[2].(float64),
				//		Price:             positionData[3].(float64),
				//		MarginFunding:     positionData[4].(float64),
				//		MarginFundingType: int64(positionData[5].(float64)),
				//		ProfitLoss:        positionData[6].(float64),
				//		ProfitLossPercent: positionData[7].(float64),
				//		LiquidationPrice:  positionData[8].(float64),
				//		Leverage:          positionData[9].(float64),
				//	}
				//	b.Websocket.DataHandler <- position
				//}
			case wsTradeExecuted, wsTradeExecutionUpdate:
				//if tradeData, ok := d[2].([]interface{}); ok && len(tradeData) > 4 {
				//	b.Websocket.DataHandler <- WebsocketTradeData{
				//		TradeID:        int64(tradeData[0].(float64)),
				//		Pair:           tradeData[1].(string),
				//		Timestamp:      int64(tradeData[2].(float64)),
				//		OrderID:        int64(tradeData[3].(float64)),
				//		AmountExecuted: tradeData[4].(float64),
				//		PriceExecuted:  tradeData[5].(float64),
				//		OrderType:      tradeData[6].(string),
				//		OrderPrice:     tradeData[7].(float64),
				//		Maker:          tradeData[8].(float64) == 1,
				//		Fee:            tradeData[9].(float64),
				//		FeeCurrency:    tradeData[10].(string),
				//	}
				//}
			case wsFundingOrderSnapshot:
				//var snapshot []WsFundingOffer
				//if snapBundle, ok := d[2].([]interface{}); ok && len(snapBundle) > 0 {
				//	if _, ok := snapBundle[0].([]interface{}); ok {
				//		for i := range snapBundle {
				//			data := snapBundle[i].([]interface{})
				//			offer := WsFundingOffer{
				//				ID:             int64(data[0].(float64)),
				//				Symbol:         data[1].(string),
				//				Created:        int64(data[2].(float64)),
				//				Updated:        int64(data[3].(float64)),
				//				Amount:         data[4].(float64),
				//				OriginalAmount: data[5].(float64),
				//				Type:           data[6].(string),
				//				Flags:          data[9].(float64),
				//				Status:         data[10].(string),
				//				Rate:           data[14].(float64),
				//				Period:         int64(data[15].(float64)),
				//				Notify:         data[16].(float64) == 1,
				//				Hidden:         data[17].(float64) == 1,
				//				Insure:         data[18].(float64) == 1,
				//				Renew:          data[19].(float64) == 1,
				//				RateReal:       data[20].(float64),
				//			}
				//			snapshot = append(snapshot, offer)
				//		}
				//		b.Websocket.DataHandler <- snapshot
				//	}
				//}
			case wsFundingOrderNew, wsFundingOrderUpdate, wsFundingOrderCancel:
				//if data, ok := d[2].([]interface{}); ok && len(data) > 0 {
				//	b.wsHandleFundingOffer(data)
				//}
			case wsFundingCreditSnapshot:
				//var snapshot []WsCredit
				//if snapBundle, ok := d[2].([]interface{}); ok && len(snapBundle) > 0 {
				//	if _, ok := snapBundle[0].([]interface{}); ok {
				//		for i := range snapBundle {
				//			data := snapBundle[i].([]interface{})
				//			credit := WsCredit{
				//				ID:           int64(data[0].(float64)),
				//				Symbol:       data[1].(string),
				//				Side:         data[2].(string),
				//				Created:      int64(data[3].(float64)),
				//				Updated:      int64(data[4].(float64)),
				//				Amount:       data[5].(float64),
				//				Flags:        data[6].(string),
				//				Status:       data[7].(string),
				//				Rate:         data[11].(float64),
				//				Period:       int64(data[12].(float64)),
				//				Opened:       int64(data[13].(float64)),
				//				LastPayout:   int64(data[14].(float64)),
				//				Notify:       data[15].(float64) == 1,
				//				Hidden:       data[16].(float64) == 1,
				//				Insure:       data[17].(float64) == 1,
				//				Renew:        data[18].(float64) == 1,
				//				RateReal:     data[19].(float64),
				//				NoClose:      data[20].(float64) == 1,
				//				PositionPair: data[21].(string),
				//			}
				//			snapshot = append(snapshot, credit)
				//		}
				//		b.Websocket.DataHandler <- snapshot
				//	}
				//}
			case wsFundingCreditNew, wsFundingCreditUpdate, wsFundingCreditCancel:
				//if data, ok := d[2].([]interface{}); ok && len(data) > 0 {
				//	b.Websocket.DataHandler <- WsCredit{
				//		ID:           int64(data[0].(float64)),
				//		Symbol:       data[1].(string),
				//		Side:         data[2].(string),
				//		Created:      int64(data[3].(float64)),
				//		Updated:      int64(data[4].(float64)),
				//		Amount:       data[5].(float64),
				//		Flags:        data[6].(string),
				//		Status:       data[7].(string),
				//		Rate:         data[11].(float64),
				//		Period:       int64(data[12].(float64)),
				//		Opened:       int64(data[13].(float64)),
				//		LastPayout:   int64(data[14].(float64)),
				//		Notify:       data[15].(float64) == 1,
				//		Hidden:       data[16].(float64) == 1,
				//		Insure:       data[17].(float64) == 1,
				//		Renew:        data[18].(float64) == 1,
				//		RateReal:     data[19].(float64),
				//		NoClose:      data[20].(float64) == 1,
				//		PositionPair: data[21].(string),
				//	}
				//}
			case wsFundingLoanSnapshot:
				//var snapshot []WsCredit
				//if snapBundle, ok := d[2].([]interface{}); ok && len(snapBundle) > 0 {
				//	if _, ok := snapBundle[0].([]interface{}); ok {
				//		for i := range snapBundle {
				//			data := snapBundle[i].([]interface{})
				//			credit := WsCredit{
				//				ID:         int64(data[0].(float64)),
				//				Symbol:     data[1].(string),
				//				Side:       data[2].(string),
				//				Created:    int64(data[3].(float64)),
				//				Updated:    int64(data[4].(float64)),
				//				Amount:     data[5].(float64),
				//				Flags:      data[6].(string),
				//				Status:     data[7].(string),
				//				Rate:       data[11].(float64),
				//				Period:     int64(data[12].(float64)),
				//				Opened:     int64(data[13].(float64)),
				//				LastPayout: int64(data[14].(float64)),
				//				Notify:     data[15].(float64) == 1,
				//				Hidden:     data[16].(float64) == 1,
				//				Insure:     data[17].(float64) == 1,
				//				Renew:      data[18].(float64) == 1,
				//				RateReal:   data[19].(float64),
				//				NoClose:    data[20].(float64) == 1,
				//			}
				//			snapshot = append(snapshot, credit)
				//		}
				//		b.Websocket.DataHandler <- snapshot
				//	}
				//}
			case wsFundingLoanNew, wsFundingLoanUpdate, wsFundingLoanCancel:
				//if data, ok := d[2].([]interface{}); ok && len(data) > 0 {
				//	b.Websocket.DataHandler <- WsCredit{
				//		ID:         int64(data[0].(float64)),
				//		Symbol:     data[1].(string),
				//		Side:       data[2].(string),
				//		Created:    int64(data[3].(float64)),
				//		Updated:    int64(data[4].(float64)),
				//		Amount:     data[5].(float64),
				//		Flags:      data[6].(string),
				//		Status:     data[7].(string),
				//		Rate:       data[11].(float64),
				//		Period:     int64(data[12].(float64)),
				//		Opened:     int64(data[13].(float64)),
				//		LastPayout: int64(data[14].(float64)),
				//		Notify:     data[15].(float64) == 1,
				//		Hidden:     data[16].(float64) == 1,
				//		Insure:     data[17].(float64) == 1,
				//		Renew:      data[18].(float64) == 1,
				//		RateReal:   data[19].(float64),
				//		NoClose:    data[20].(float64) == 1,
				//	}
				//}
			case wsWalletSnapshot:
				//var snapshot []WsWallet
				//if snapBundle, ok := d[2].([]interface{}); ok && len(snapBundle) > 0 {
				//	if _, ok := snapBundle[0].([]interface{}); ok {
				//		for i := range snapBundle {
				//			data := snapBundle[i].([]interface{})
				//			var balanceAvailable float64
				//			if _, ok := data[4].(float64); ok {
				//				balanceAvailable = data[4].(float64)
				//			}
				//			wallet := WsWallet{
				//				Type:              data[0].(string),
				//				Currency:          data[1].(string),
				//				Balance:           data[2].(float64),
				//				UnsettledInterest: data[3].(float64),
				//				BalanceAvailable:  balanceAvailable,
				//			}
				//			snapshot = append(snapshot, wallet)
				//		}
				//		b.Websocket.DataHandler <- snapshot
				//	}
				//}
			case wsWalletUpdate:
				//if data, ok := d[2].([]interface{}); ok && len(data) > 0 {
				//	var balanceAvailable float64
				//	if _, ok := data[4].(float64); ok {
				//		balanceAvailable = data[4].(float64)
				//	}
				//	b.Websocket.DataHandler <- WsWallet{
				//		Type:              data[0].(string),
				//		Currency:          data[1].(string),
				//		Balance:           data[2].(float64),
				//		UnsettledInterest: data[3].(float64),
				//		BalanceAvailable:  balanceAvailable,
				//	}
				//}
			case wsBalanceUpdate:
				//if data, ok := d[2].([]interface{}); ok && len(data) > 0 {
				//	b.Websocket.DataHandler <- WsBalanceInfo{
				//		TotalAssetsUnderManagement: data[0].(float64),
				//		NetAssetsUnderManagement:   data[1].(float64),
				//	}
				//}
			case wsMarginInfoUpdate:
				//if data, ok := d[2].([]interface{}); ok && len(data) > 0 {
				//	if data[0].(string) == "base" {
				//		if infoBase, ok := d[2].([]interface{}); ok && len(infoBase) > 0 {
				//			baseData := data[1].([]interface{})
				//			b.Websocket.DataHandler <- WsMarginInfoBase{
				//				UserProfitLoss: baseData[0].(float64),
				//				UserSwaps:      baseData[1].(float64),
				//				MarginBalance:  baseData[2].(float64),
				//				MarginNet:      baseData[3].(float64),
				//			}
				//		}
				//	}
				//}
			case wsFundingInfoUpdate:
				//if data, ok := d[2].([]interface{}); ok && len(data) > 0 {
				//	if data[0].(string) == "sym" {
				//		symbolData := data[1].([]interface{})
				//		b.Websocket.DataHandler <- WsFundingInfo{
				//			YieldLoan:    symbolData[0].(float64),
				//			YieldLend:    symbolData[1].(float64),
				//			DurationLoan: symbolData[2].(float64),
				//			DurationLend: symbolData[3].(float64),
				//		}
				//	}
				//}
			case wsFundingTradeExecuted, wsFundingTradeUpdate:
				//if data, ok := d[2].([]interface{}); ok && len(data) > 0 {
				//	b.Websocket.DataHandler <- WsFundingTrade{
				//		ID:         int64(data[0].(float64)),
				//		Symbol:     data[1].(string),
				//		MTSCreated: int64(data[2].(float64)),
				//		OfferID:    int64(data[3].(float64)),
				//		Amount:     data[4].(float64),
				//		Rate:       data[5].(float64),
				//		Period:     int64(data[6].(float64)),
				//		Maker:      data[7].(float64) == 1,
				//	}
				//}
			default:
				b.Websocket.DataHandler <- stream.UnhandledMessageWarning{
					Message: b.Name + stream.UnhandledMessage + string(respRaw),
				}
				return nil
			}
		}
	default:
		var isChannelExist bool
		if result.ID > 0 {
			isChannelExist = b.Websocket.Match.IncomingWithData(result.ID, respRaw)
			//fmt.Println("EXIST", isChannelExist, string(respRaw))
		}

		if !isChannelExist && result.ID > 0 {
			var showMessage bool
			switch result.Result.(type) {
			case map[string]interface{}:
				result := result.Result.(map[string]interface{})
				if status, ok := result["status"]; ok {
					switch status {
					case "success":
					default:
						showMessage = true
					}
				}
			}

			if showMessage {
				return fmt.Errorf("can't send ws incoming data to Matched channel with RequestID: %d, %+v",
					result.ID, (result.Result.(map[string]interface{}))["status"].(string))
			}
		}
	}
	return nil
}

// WsInsertSnapshot add the initial orderbook snapshot when subscribed to a
// channel
func (b *Whitebit) WsInsertSnapshot(p currency.Pair, assetType asset.Item, books []WebsocketBook, fundingRate bool) error {
	if len(books) == 0 {
		return errors.New("bitfinex.go error - no orderbooks submitted")
	}
	var book orderbook.Base
	for i := range books {
		item := orderbook.Item{
			ID:     books[i].ID,
			Amount: books[i].Amount,
			Price:  books[i].Price,
			Period: books[i].Period,
		}
		if fundingRate {
			if item.Amount < 0 {
				item.Amount *= -1
				book.Bids = append(book.Bids, item)
			} else {
				book.Asks = append(book.Asks, item)
			}
		} else {
			if books[i].Amount > 0 {
				book.Bids = append(book.Bids, item)
			} else {
				item.Amount *= -1
				book.Asks = append(book.Asks, item)
			}
		}
	}

	book.AssetType = assetType
	book.Pair = p
	book.ExchangeName = b.Name
	book.NotAggregated = true
	book.HasChecksumValidation = true
	book.IsFundingRate = fundingRate
	book.VerificationBypass = b.OrderbookVerificationBypass
	return b.Websocket.Orderbook.LoadSnapshot(&book)
}

// WsUpdateOrderbook updates the orderbook list, removing and adding to the
// orderbook sides
func (b *Whitebit) WsUpdateOrderbook(p currency.Pair, assetType asset.Item, book []WebsocketBook, channelID int, sequenceNo int64, fundingRate bool) error {
	orderbookUpdate := buffer.Update{Asset: assetType, Pair: p}

	for i := range book {
		item := orderbook.Item{
			ID:     book[i].ID,
			Amount: book[i].Amount,
			Price:  book[i].Price,
			Period: book[i].Period,
		}

		if book[i].Price > 0 {
			orderbookUpdate.Action = buffer.UpdateInsert
			if fundingRate {
				if book[i].Amount < 0 {
					item.Amount *= -1
					orderbookUpdate.Bids = append(orderbookUpdate.Bids, item)
				} else {
					orderbookUpdate.Asks = append(orderbookUpdate.Asks, item)
				}
			} else {
				if book[i].Amount > 0 {
					orderbookUpdate.Bids = append(orderbookUpdate.Bids, item)
				} else {
					item.Amount *= -1
					orderbookUpdate.Asks = append(orderbookUpdate.Asks, item)
				}
			}
		} else {
			orderbookUpdate.Action = buffer.Delete
			if fundingRate {
				if book[i].Amount == 1 {
					// delete bid
					orderbookUpdate.Asks = append(orderbookUpdate.Asks, item)
				} else {
					// delete ask
					orderbookUpdate.Bids = append(orderbookUpdate.Bids, item)
				}
			} else {
				if book[i].Amount == 1 {
					// delete bid
					orderbookUpdate.Bids = append(orderbookUpdate.Bids, item)
				} else {
					// delete ask
					orderbookUpdate.Asks = append(orderbookUpdate.Asks, item)
				}
			}
		}
	}

	cMtx.Lock()
	checkme := checksumStore[channelID]
	if checkme == nil {
		cMtx.Unlock()
		return b.Websocket.Orderbook.Update(&orderbookUpdate)
	}
	checksumStore[channelID] = nil
	cMtx.Unlock()

	if checkme.Sequence+1 == sequenceNo {
		// Sequence numbers get dropped, if checksum is not in line with
		// sequence, do not check.
		ob := b.Websocket.Orderbook.GetOrderbook(p, assetType)
		if ob == nil {
			return fmt.Errorf("cannot calculate websocket checksum: book not found for %s %s",
				p,
				assetType)
		}

		err := validateCRC32(ob, checkme.Token)
		if err != nil {
			return err
		}
	}

	return b.Websocket.Orderbook.Update(&orderbookUpdate)
}

// GenerateDefaultSubscriptions Adds default subscriptions to websocket to be handled by ManageSubscriptions()
func (b *Whitebit) GenerateDefaultSubscriptions() ([]stream.ChannelSubscription, error) {
	var channels = []string{
		wsBook,
		//wsTrades,
		//wsTicker,
	}

	var subscriptions []stream.ChannelSubscription
	assets := b.GetAssetTypes()
	for i := range assets {
		enabledPairs, err := b.GetEnabledPairs(assets[i])
		if err != nil {
			return nil, err
		}

		allPairs := make(map[string]interface{})
		for j := range channels {
			for k := range enabledPairs {
				params := make(map[string]interface{})
				if channels[j] == wsBook {
					params["pair"] = currency.NewPairWithDelimiter(enabledPairs[k].Base.String(), enabledPairs[k].Quote.String(), "_")
					params["limit"] = 100
					params["interval"] = "0"
					params["flag"] = true
				}

				if channels[j] == wsTrades {
					//p, err := currency.NewPairDelimiter(enabledPairs[k], "-")
					p :=currency.NewPairWithDelimiter(enabledPairs[k].Base.String(), enabledPairs[k].Quote.String(), "-")
					fmt.Println("P:", p)
					allPairs[enabledPairs[k].String()] = p //enabledPairs[k].String()
					continue
				}

				subscriptions = append(subscriptions, stream.ChannelSubscription{
					Channel:  channels[j],
					Currency: enabledPairs[k],
					Params:   params,
					Asset:    assets[i],
				})
			}

			if channels[j] == wsTrades {
				subscriptions = append(subscriptions, stream.ChannelSubscription{
					Channel:  channels[j],
					Currency: currency.NewPair(currency.BTC, currency.USDT), // dummy pair
					Params:   allPairs,
					Asset:    assets[i],
				})
			}
		}
	}

	return subscriptions, nil
}

// Subscribe sends a websocket message to receive data from the channel
func (b *Whitebit) Subscribe(channelsToSubscribe []stream.ChannelSubscription) error {
	//fmt.Println("channelsToSubscribe:", channelsToSubscribe)
	var errs common.Errors

	for i := range channelsToSubscribe {
		var prm []interface{}

		//for _, v := range channelsToSubscribe[i].Params {
		//	prm = append(prm, channelsToSubscribe[i].Params["pair"])
		//}

		prm = append(prm,
			channelsToSubscribe[i].Params["pair"],
			channelsToSubscribe[i].Params["limit"],
			channelsToSubscribe[i].Params["interval"],
			channelsToSubscribe[i].Params["flag"],
			)

		req := WsRequest{
			ID: time.Now().UnixNano() / 1000,
			Method: channelsToSubscribe[i].Channel,
			Params: prm,
		}

		fmt.Printf("channelsToSubscribe: %+v\n", req)
		err := b.Websocket.Conn.SendJSONMessage(req)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		b.Websocket.AddSuccessfulSubscriptions(channelsToSubscribe[i])
	}
	if errs != nil {
		return errs
	}
	return nil
}

// Unsubscribe sends a websocket message to stop receiving data from the channel
func (b *Whitebit) Unsubscribe(channelsToUnsubscribe []stream.ChannelSubscription) error {
	var errs common.Errors
	for i := range channelsToUnsubscribe {
		req := make(map[string]interface{})
		req["event"] = "unsubscribe"
		req["channel"] = channelsToUnsubscribe[i].Channel

		for k, v := range channelsToUnsubscribe[i].Params {
			req[k] = v
		}

		err := b.Websocket.Conn.SendJSONMessage(req)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		b.Websocket.RemoveSuccessfulUnsubscriptions(channelsToUnsubscribe[i])
	}
	if errs != nil {
		return errs
	}
	return nil
}

// WsSendAuth sends a autheticated event payload
func (b *Whitebit) WsSendAuth() error {
	//if !b.GetAuthenticatedAPISupport(exchange.WebsocketAuthentication) {
	//	return fmt.Errorf("%v AuthenticatedWebsocketAPISupport not enabled",
	//		b.Name)
	//}

	request := WsRequest{
		ID:       0,
		Method:   "authorize",
		Params: []interface{}{
			authToken,
			"public",
		},
	}
	err := b.Websocket.Conn.SendJSONMessage(request)
	if err != nil {
		fmt.Println("WsSendAuth FAIL")
		b.Websocket.SetCanUseAuthenticatedEndpoints(false)
		return err
	}

	fmt.Println("WsSendAuth OK")
	//time.Sleep(time.Second * 2)

	request = WsRequest{
		ID:       0,
		Method:   "balanceSpot_request",
		Params: []interface{}{
			"ETH",
			"BTC",
		},
	}
	err = b.Websocket.Conn.SendJSONMessage(request)
	if err != nil {
		fmt.Println("balanceSpot_request FAIL")
		b.Websocket.SetCanUseAuthenticatedEndpoints(false)
		return err
	}

	return nil
}

// WsAddSubscriptionChannel adds a new subscription channel to the
// WebsocketSubdChannels map in bitfinex.go (Bitfinex struct)
func (b *Whitebit) WsAddSubscriptionChannel(chanID int, channel, pair string) {
	chanInfo := WebsocketChanInfo{Pair: pair, Channel: channel}
	b.WebsocketSubdChannels[chanID] = chanInfo

	if b.Verbose {
		log.Debugf(log.ExchangeSys,
			"%s Subscribed to Channel: %s Pair: %s ChannelID: %d\n",
			b.Name,
			channel,
			pair,
			chanID)
	}
}


func validateCRC32(book *orderbook.Base, token int) error {
	// Order ID's need to be sub-sorted in ascending order, this needs to be
	// done on the main book to ensure that we do not cut price levels out below
	reOrderByID(book.Bids)
	reOrderByID(book.Asks)

	// RO precision calculation is based on order ID's and amount values
	var bids, asks []orderbook.Item
	for i := 0; i < 25; i++ {
		if i < len(book.Bids) {
			bids = append(bids, book.Bids[i])
		}
		if i < len(book.Asks) {
			asks = append(asks, book.Asks[i])
		}
	}

	// ensure '-' (negative amount) is passed back to string buffer as
	// this is needed for calcs - These get swapped if funding rate
	bidmod := float64(1)
	if book.IsFundingRate {
		bidmod = -1
	}

	askMod := float64(-1)
	if book.IsFundingRate {
		askMod = 1
	}

	var check strings.Builder
	for i := 0; i < 25; i++ {
		if i < len(bids) {
			check.WriteString(strconv.FormatInt(bids[i].ID, 10))
			check.WriteString(":")
			check.WriteString(strconv.FormatFloat(bidmod*bids[i].Amount, 'f', -1, 64))
			check.WriteString(":")
		}

		if i < len(asks) {
			check.WriteString(strconv.FormatInt(asks[i].ID, 10))
			check.WriteString(":")
			check.WriteString(strconv.FormatFloat(askMod*asks[i].Amount, 'f', -1, 64))
			check.WriteString(":")
		}
	}

	checksumStr := strings.TrimSuffix(check.String(), ":")
	checksum := crc32.ChecksumIEEE([]byte(checksumStr))
	if checksum == uint32(token) {
		return nil
	}
	return fmt.Errorf("invalid checksum for %s %s: calculated [%d] does not match [%d]",
		book.AssetType,
		book.Pair,
		checksum,
		uint32(token))
}

// reOrderByID sub sorts orderbook items by its corresponding ID when price
// levels are the same. TODO: Deprecate and shift to buffer level insertion
// based off ascending ID.
func reOrderByID(depth []orderbook.Item) {
subSort:
	for x := 0; x < len(depth); {
		var subset []orderbook.Item
		// Traverse forward elements
		for y := x + 1; y < len(depth); y++ {
			if depth[x].Price == depth[y].Price &&
				// Period matching is for funding rates, this was undocumented
				// but these need to be matched with price for the correct ID
				// alignment
				depth[x].Period == depth[y].Period {
				// Append element to subset when price match occurs
				subset = append(subset, depth[y])
				// Traverse next
				continue
			}
			if len(subset) != 0 {
				// Append root element
				subset = append(subset, depth[x])
				// Sort IDs by ascending
				sort.Slice(subset, func(i, j int) bool {
					return subset[i].ID < subset[j].ID
				})
				// Re-align elements with sorted ID subset
				for z := range subset {
					depth[x+z] = subset[z]
				}
			}
			// When price is not matching change checked element to root
			x = y
			continue subSort
		}
		break
	}
}

// wsGetAccountBalance WS authenticated get balances. If no assets passed - return all assets
func (b *Whitebit) wsGetAccountBalance(orderID int64, assets []string) (bals map[string]Balance, err  error) {
	bals = make(map[string]Balance)

	request := WsRequest{
		ID:       orderID,
		Method:   "balanceSpot_request",
		Params: []interface{}{
			//"ETH",
			//"BTC",
		},
	}

	for i := range assets {
		request.Params = append(request.Params, assets[i])
	}

	resp, err := b.Websocket.Conn.SendMessageReturnResponse(orderID, request)
	if err != nil {
		return
	}
	if resp == nil {
		return bals, fmt.Errorf("%v - wsGetAccountBalance failed", b.Name)
	}

	var responseData WsRequest
	err = json.Unmarshal(resp, &responseData)
	if err != nil {
		return
	}

	if responseData.Error != nil {
		return bals, fmt.Errorf("%v - wsGetAccountBalance failed: %s", b.Name, responseData.Error)
	}

	type Bbb struct {
		Freeze    float64 `json:"freeze,string"`
		Available float64 `json:"available,string"`
	}

	type X map[string]map[string]string

	if bl, ok := responseData.Result.(map[string]interface{}); ok {
		for k, v := range bl {
			if bs, ok := v.(map[string]interface{}); ok {
				var available, freeze float64
				if available, err = strconv.ParseFloat(bs["available"].(string), 64); err != nil {
					return bals, fmt.Errorf("%v - wsGetAccountBalance failed: %s", b.Name, err)
				}

				if freeze, err = strconv.ParseFloat(bs["freeze"].(string), 64); err != nil {
					return bals, fmt.Errorf("%v - wsGetAccountBalance failed: %s", b.Name, err)
				}

				bals[k] = Balance{
					Available: available,
					Freeze: freeze,
				}
			}
		}

		return
	}

	return bals, fmt.Errorf("%v - wsGetAccountBalance failed", b.Name)
}

// wsGetExecutedOrders WS authenticated get Closed Orders. // Side 1 - sell, 2 - bid
func (b *Whitebit) wsGetExecutedOrders(orderID int64, pair string, limit, offset int64) (orders []Order, err  error) {
	request := WsRequest{
		ID:       orderID,
		Method:   "ordersExecuted_request",
		Params: []interface{}{
			map[string]interface{}{
				"market": pair, // market
				"order_types": []interface{}{1, 2},   // 1 - Limit, 2 - Market
			},
			offset,
			limit,
		},
	}

	resp, err := b.Websocket.Conn.SendMessageReturnResponse(orderID, request)
	if err != nil {
		return
	}
	//if resp == nil {
	//	return orders, fmt.Errorf("%v - wsGetOpenOrders failed", b.Name)
	//}

	//fmt.Printf("%s WWW: %+v\n", pair, string(resp))

	var responseData WsOrdersExecuted
	err = json.Unmarshal(resp, &responseData)
	if err != nil {
		return
	}

	if responseData.Error != nil {
		return orders, fmt.Errorf("%v - wsGetExecutedOrders failed: %s", b.Name, responseData.Error)
	}

	//fmt.Printf("VVV: %+v\n", responseData.Result)
	for i := range responseData.Result.Records {
		orders = append(orders, Order{
			ID: responseData.Result.Records[i].OrderID,
			Ctime: responseData.Result.Records[i].CTime,
			Ftime: responseData.Result.Records[i].FTime,
			Market: responseData.Result.Records[i].Market,
			Source: responseData.Result.Records[i].Source,
			Type: fmt.Sprint(responseData.Result.Records[i].Type),
			Side: fmt.Sprint(responseData.Result.Records[i].Side),
			Price: responseData.Result.Records[i].Price,
			Amount: responseData.Result.Records[i].Amount,
			TakerFee: responseData.Result.Records[i].TakerFee,
			MakerFee: responseData.Result.Records[i].MakerFee,
			DealStock: responseData.Result.Records[i].DealStock,
			DealMoney: responseData.Result.Records[i].DealMoney,
			DealFee: responseData.Result.Records[i].DealFee,
			ClientOrderId: responseData.Result.Records[i].ClientOrderId,
		})
	}


	//type Bbb struct {
	//	Freeze    float64 `json:"freeze,string"`
	//	Available float64 `json:"available,string"`
	//}

	//type X map[string]map[string]string
	//
	//if bl, ok := responseData.Result.(map[string]interface{}); ok {
	//	for k, v := range bl {
	//		if bs, ok := v.(map[string]interface{}); ok {
	//			var available, freeze float64
	//			if available, err = strconv.ParseFloat(bs["available"].(string), 64); err != nil {
	//				return orders, fmt.Errorf("%v - wsGetOpenOrders failed: %s", b.Name, err)
	//			}
	//
	//			if freeze, err = strconv.ParseFloat(bs["freeze"].(string), 64); err != nil {
	//				return orders, fmt.Errorf("%v - wsGetOpenOrders failed: %s", b.Name, err)
	//			}
	//
	//			orders = Balance{
	//				Available: available,
	//				Freeze: freeze,
	//			}
	//		}
	//	}
	//
	//	return
	//}

	return orders, nil
}