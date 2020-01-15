package bittrex

import (
	"log"
	"os"
	"testing"

	"github.com/vazha/gocryptotrader/common"
	"github.com/vazha/gocryptotrader/config"
	"github.com/vazha/gocryptotrader/currency"
	exchange "github.com/vazha/gocryptotrader/exchanges"
	"github.com/vazha/gocryptotrader/exchanges/order"
	"github.com/vazha/gocryptotrader/exchanges/withdraw"
)

// Please supply you own test keys here to run better tests.
const (
	apiKey                  = ""
	apiSecret               = ""
	canManipulateRealOrders = false
)

var b Bittrex

func TestMain(m *testing.M) {
	b.SetDefaults()
	cfg := config.GetConfig()
	err := cfg.LoadConfig("../../testdata/configtest.json", true)
	if err != nil {
		log.Fatal("Bittrex load config error", err)
	}
	bConfig, err := cfg.GetExchangeConfig("Bittrex")
	if err != nil {
		log.Fatal("Bittrex Setup() init error")
	}
	bConfig.API.Credentials.Key = apiKey
	bConfig.API.Credentials.Secret = apiSecret
	bConfig.API.AuthenticatedSupport = true

	err = b.Setup(bConfig)
	if err != nil {
		log.Fatal("Bittrex setup error", err)
	}

	if !b.IsEnabled() || !b.API.AuthenticatedSupport ||
		b.Verbose || len(b.BaseCurrencies) < 1 {
		log.Fatal("Bittrex Setup values not set correctly")
	}

	os.Exit(m.Run())
}

func TestGetMarkets(t *testing.T) {
	t.Parallel()
	_, err := b.GetMarkets()
	if err != nil {
		t.Errorf("Bittrex - GetMarkets() error: %s", err)
	}
}

func TestGetCurrencies(t *testing.T) {
	t.Parallel()
	_, err := b.GetCurrencies()
	if err != nil {
		t.Errorf("Bittrex - GetCurrencies() error: %s", err)
	}
}

func TestGetTicker(t *testing.T) {
	t.Parallel()
	btc := "btc-ltc"

	_, err := b.GetTicker(btc)
	if err != nil {
		t.Errorf("Bittrex - GetTicker() error: %s", err)
	}
}

func TestGetMarketSummaries(t *testing.T) {
	t.Parallel()
	_, err := b.GetMarketSummaries()
	if err != nil {
		t.Errorf("Bittrex - GetMarketSummaries() error: %s", err)
	}
}

func TestGetMarketSummary(t *testing.T) {
	t.Parallel()
	pairOne := "BTC-LTC"

	_, err := b.GetMarketSummary(pairOne)
	if err != nil {
		t.Errorf("Bittrex - GetMarketSummary() error: %s", err)
	}
}

func TestGetOrderbook(t *testing.T) {
	t.Parallel()

	_, err := b.GetOrderbook("btc-ltc")
	if err != nil {
		t.Errorf("Bittrex - GetOrderbook() error: %s", err)
	}
}

func TestGetMarketHistory(t *testing.T) {
	t.Parallel()

	_, err := b.GetMarketHistory("btc-ltc")
	if err != nil {
		t.Errorf("Bittrex - GetMarketHistory() error: %s", err)
	}
}

func TestPlaceBuyLimit(t *testing.T) {
	t.Parallel()

	_, err := b.PlaceBuyLimit("btc-ltc", 1, 1)
	if err == nil {
		t.Error("Bittrex - PlaceBuyLimit() Expected error")
	}
}

func TestPlaceSellLimit(t *testing.T) {
	t.Parallel()

	_, err := b.PlaceSellLimit("btc-ltc", 1, 1)
	if err == nil {
		t.Error("Bittrex - PlaceSellLimit() Expected error")
	}
}

func TestGetOpenOrders(t *testing.T) {
	t.Parallel()

	_, err := b.GetOpenOrders("")
	if err == nil {
		t.Error("Bittrex - GetOrder() Expected error")
	}
	_, err = b.GetOpenOrders("btc-ltc")
	if err == nil {
		t.Error("Bittrex - GetOrder() Expected error")
	}
}

func TestCancelExistingOrder(t *testing.T) {
	t.Parallel()

	_, err := b.CancelExistingOrder("blaaaaaaa")
	if err == nil {
		t.Error("Bittrex - CancelExistingOrder() Expected error")
	}
}

func TestGetAccountBalances(t *testing.T) {
	t.Parallel()

	_, err := b.GetAccountBalances()
	if err == nil {
		t.Error("Bittrex - GetAccountBalances() Expected error")
	}
}

func TestGetAccountBalanceByCurrency(t *testing.T) {
	t.Parallel()

	_, err := b.GetAccountBalanceByCurrency("btc")
	if err == nil {
		t.Error("Bittrex - GetAccountBalanceByCurrency() Expected error")
	}
}

func TestGetOrder(t *testing.T) {
	t.Parallel()

	_, err := b.GetOrder("0cb4c4e4-bdc7-4e13-8c13-430e587d2cc1")
	if err == nil {
		t.Error("Bittrex - GetOrder() Expected error")
	}
	_, err = b.GetOrder("")
	if err == nil {
		t.Error("Bittrex - GetOrder() Expected error")
	}
}

func TestGetOrderHistoryForCurrency(t *testing.T) {
	t.Parallel()

	_, err := b.GetOrderHistoryForCurrency("")
	if err == nil {
		t.Error("Bittrex - GetOrderHistory() Expected error")
	}
	_, err = b.GetOrderHistoryForCurrency("btc-ltc")
	if err == nil {
		t.Error("Bittrex - GetOrderHistory() Expected error")
	}
}

func TestGetwithdrawalHistory(t *testing.T) {
	t.Parallel()

	_, err := b.GetWithdrawalHistory("")
	if err == nil {
		t.Error("Bittrex - GetWithdrawalHistory() Expected error")
	}
	_, err = b.GetWithdrawalHistory("btc-ltc")
	if err == nil {
		t.Error("Bittrex - GetWithdrawalHistory() Expected error")
	}
}

func TestGetDepositHistory(t *testing.T) {
	t.Parallel()

	_, err := b.GetDepositHistory("")
	if err == nil {
		t.Error("Bittrex - GetDepositHistory() Expected error")
	}
	_, err = b.GetDepositHistory("btc-ltc")
	if err == nil {
		t.Error("Bittrex - GetDepositHistory() Expected error")
	}
}

func setFeeBuilder() *exchange.FeeBuilder {
	return &exchange.FeeBuilder{
		Amount:        1,
		FeeType:       exchange.CryptocurrencyTradeFee,
		Pair:          currency.NewPair(currency.BTC, currency.LTC),
		PurchasePrice: 1,
	}
}

// TestGetFeeByTypeOfflineTradeFee logic test
func TestGetFeeByTypeOfflineTradeFee(t *testing.T) {
	var feeBuilder = setFeeBuilder()
	b.GetFeeByType(feeBuilder)
	if !areTestAPIKeysSet() {
		if feeBuilder.FeeType != exchange.OfflineTradeFee {
			t.Errorf("Expected %v, received %v", exchange.OfflineTradeFee, feeBuilder.FeeType)
		}
	} else {
		if feeBuilder.FeeType != exchange.CryptocurrencyTradeFee {
			t.Errorf("Expected %v, received %v", exchange.CryptocurrencyTradeFee, feeBuilder.FeeType)
		}
	}
}

func TestGetFee(t *testing.T) {
	var feeBuilder = setFeeBuilder()
	// CryptocurrencyTradeFee Basic
	if resp, err := b.GetFee(feeBuilder); resp != float64(0.0025) || err != nil {
		t.Error(err)
		t.Errorf("GetFee() error. Expected: %f, Received: %f", float64(0.0025), resp)
	}

	// CryptocurrencyTradeFee High quantity
	feeBuilder = setFeeBuilder()
	feeBuilder.Amount = 1000
	feeBuilder.PurchasePrice = 1000
	if resp, err := b.GetFee(feeBuilder); resp != float64(2500) || err != nil {
		t.Errorf("GetFee() error. Expected: %f, Received: %f", float64(2500), resp)
		t.Error(err)
	}

	// CryptocurrencyTradeFee IsMaker
	feeBuilder = setFeeBuilder()
	feeBuilder.IsMaker = true
	if resp, err := b.GetFee(feeBuilder); resp != float64(0.0025) || err != nil {
		t.Errorf("GetFee() error. Expected: %f, Received: %f", float64(0.0025), resp)
		t.Error(err)
	}

	// CryptocurrencyTradeFee Negative purchase price
	feeBuilder = setFeeBuilder()
	feeBuilder.PurchasePrice = -1000
	if resp, err := b.GetFee(feeBuilder); resp != float64(0) || err != nil {
		t.Errorf("GetFee() error. Expected: %f, Received: %f", float64(0), resp)
		t.Error(err)
	}

	// CryptocurrencyWithdrawalFee Basic
	feeBuilder = setFeeBuilder()
	feeBuilder.FeeType = exchange.CryptocurrencyWithdrawalFee
	if resp, err := b.GetFee(feeBuilder); resp != float64(0.0005) || err != nil {
		t.Errorf("GetFee() error. Expected: %f, Received: %f", float64(0.0005), resp)
		t.Error(err)
	}

	// CyptocurrencyDepositFee Basic
	feeBuilder = setFeeBuilder()
	feeBuilder.FeeType = exchange.CyptocurrencyDepositFee
	if resp, err := b.GetFee(feeBuilder); resp != float64(0) || err != nil {
		t.Errorf("GetFee() error. Expected: %f, Received: %f", float64(0), resp)
		t.Error(err)
	}

	// InternationalBankDepositFee Basic
	feeBuilder = setFeeBuilder()
	feeBuilder.FeeType = exchange.InternationalBankDepositFee
	feeBuilder.FiatCurrency = currency.HKD
	if resp, err := b.GetFee(feeBuilder); resp != float64(0) || err != nil {
		t.Errorf("GetFee() error. Expected: %f, Received: %f", float64(0), resp)
		t.Error(err)
	}

	// InternationalBankWithdrawalFee Basic
	feeBuilder = setFeeBuilder()
	feeBuilder.FeeType = exchange.InternationalBankWithdrawalFee
	feeBuilder.FiatCurrency = currency.HKD
	if resp, err := b.GetFee(feeBuilder); resp != float64(0) || err != nil {
		t.Errorf("GetFee() error. Expected: %f, Received: %f", float64(0), resp)
		t.Error(err)
	}
}

func TestFormatWithdrawPermissions(t *testing.T) {
	expectedResult := exchange.AutoWithdrawCryptoWithAPIPermissionText + " & " + exchange.NoFiatWithdrawalsText
	withdrawPermissions := b.FormatWithdrawPermissions()
	if withdrawPermissions != expectedResult {
		t.Errorf("Expected: %s, Received: %s", expectedResult, withdrawPermissions)
	}
}

func TestGetActiveOrders(t *testing.T) {
	var getOrdersRequest = order.GetOrdersRequest{
		OrderType: order.AnyType,
		Currencies: []currency.Pair{currency.NewPair(currency.LTC,
			currency.BTC)},
	}

	getOrdersRequest.Currencies[0].Delimiter = "-"

	_, err := b.GetActiveOrders(&getOrdersRequest)
	if areTestAPIKeysSet() && err != nil {
		t.Errorf("Could not get open orders: %s", err)
	} else if !areTestAPIKeysSet() && err == nil {
		t.Error("Expecting an error when no keys are set")
	}
}

func TestGetOrderHistory(t *testing.T) {
	var getOrdersRequest = order.GetOrdersRequest{
		OrderType: order.AnyType,
	}

	_, err := b.GetOrderHistory(&getOrdersRequest)
	if areTestAPIKeysSet() && err != nil {
		t.Errorf("Could not get order history: %s", err)
	} else if !areTestAPIKeysSet() && err == nil {
		t.Error("Expecting an error when no keys are set")
	}
}

// Any tests below this line have the ability to impact your orders on the exchange. Enable canManipulateRealOrders to run them
// ----------------------------------------------------------------------------------------------------------------------------
func areTestAPIKeysSet() bool {
	return b.ValidateAPICredentials()
}

func TestSubmitOrder(t *testing.T) {
	if areTestAPIKeysSet() && !canManipulateRealOrders {
		t.Skip("API keys set, canManipulateRealOrders false, skipping test")
	}

	var orderSubmission = &order.Submit{
		Pair: currency.Pair{
			Delimiter: "-",
			Base:      currency.BTC,
			Quote:     currency.LTC,
		},
		OrderSide: order.Buy,
		OrderType: order.Limit,
		Price:     1,
		Amount:    1,
		ClientID:  "meowOrder",
	}
	response, err := b.SubmitOrder(orderSubmission)
	if areTestAPIKeysSet() && (err != nil || !response.IsOrderPlaced) {
		t.Errorf("Order failed to be placed: %v", err)
	} else if !areTestAPIKeysSet() && err == nil {
		t.Error("Expecting an error when no keys are set")
	}
}

func TestCancelExchangeOrder(t *testing.T) {
	if areTestAPIKeysSet() && !canManipulateRealOrders {
		t.Skip("API keys set, canManipulateRealOrders false, skipping test")
	}

	currencyPair := currency.NewPair(currency.LTC, currency.BTC)
	var orderCancellation = &order.Cancel{
		OrderID:       "1",
		WalletAddress: "1F5zVDgNjorJ51oGebSvNCrSAHpwGkUdDB",
		AccountID:     "1",
		CurrencyPair:  currencyPair,
	}

	err := b.CancelOrder(orderCancellation)
	if !areTestAPIKeysSet() && err == nil {
		t.Error("Expecting an error when no keys are set")
	}
	if areTestAPIKeysSet() && err != nil {
		t.Errorf("Could not cancel orders: %v", err)
	}
}

func TestCancelAllExchangeOrders(t *testing.T) {
	if areTestAPIKeysSet() && !canManipulateRealOrders {
		t.Skip("API keys set, canManipulateRealOrders false, skipping test")
	}

	currencyPair := currency.NewPair(currency.LTC, currency.BTC)
	var orderCancellation = &order.Cancel{
		OrderID:       "1",
		WalletAddress: "1F5zVDgNjorJ51oGebSvNCrSAHpwGkUdDB",
		AccountID:     "1",
		CurrencyPair:  currencyPair,
	}

	resp, err := b.CancelAllOrders(orderCancellation)

	if !areTestAPIKeysSet() && err == nil {
		t.Error("Expecting an error when no keys are set")
	}
	if areTestAPIKeysSet() && err != nil {
		t.Errorf("Could not cancel orders: %v", err)
	}

	if len(resp.Status) > 0 {
		t.Errorf("%v orders failed to cancel", len(resp.Status))
	}
}

func TestModifyOrder(t *testing.T) {
	if areTestAPIKeysSet() && !canManipulateRealOrders {
		t.Skip("API keys set, canManipulateRealOrders false, skipping test")
	}
	_, err := b.ModifyOrder(&order.Modify{})
	if err == nil {
		t.Error("ModifyOrder() Expected error")
	}
}

func TestWithdraw(t *testing.T) {
	withdrawCryptoRequest := withdraw.CryptoRequest{
		GenericInfo: withdraw.GenericInfo{
			Amount:      -1,
			Currency:    currency.BTC,
			Description: "WITHDRAW IT ALL",
		},
		Address: "1F5zVDgNjorJ51oGebSvNCrSAHpwGkUdDB",
	}

	if areTestAPIKeysSet() && !canManipulateRealOrders {
		t.Skip("API keys set, canManipulateRealOrders false, skipping test")
	}

	_, err := b.WithdrawCryptocurrencyFunds(&withdrawCryptoRequest)
	if !areTestAPIKeysSet() && err == nil {
		t.Error("Expecting an error when no keys are set")
	}
	if areTestAPIKeysSet() && err != nil {
		t.Errorf("Withdraw failed to be placed: %v", err)
	}
}

func TestWithdrawFiat(t *testing.T) {
	if areTestAPIKeysSet() && !canManipulateRealOrders {
		t.Skip("API keys set, canManipulateRealOrders false, skipping test")
	}

	var withdrawFiatRequest = withdraw.FiatRequest{}

	_, err := b.WithdrawFiatFunds(&withdrawFiatRequest)
	if err != common.ErrFunctionNotSupported {
		t.Errorf("Expected '%v', received: '%v'", common.ErrFunctionNotSupported, err)
	}
}

func TestWithdrawInternationalBank(t *testing.T) {
	if areTestAPIKeysSet() && !canManipulateRealOrders {
		t.Skip("API keys set, canManipulateRealOrders false, skipping test")
	}

	var withdrawFiatRequest = withdraw.FiatRequest{}

	_, err := b.WithdrawFiatFundsToInternationalBank(&withdrawFiatRequest)
	if err != common.ErrFunctionNotSupported {
		t.Errorf("Expected '%v', received: '%v'", common.ErrFunctionNotSupported, err)
	}
}

func TestGetDepositAddress(t *testing.T) {
	if areTestAPIKeysSet() {
		_, err := b.GetDepositAddress(currency.BTC, "")
		if err != nil {
			t.Error("GetDepositAddress() error", err)
		}
	} else {
		_, err := b.GetDepositAddress(currency.BTC, "")
		if err == nil {
			t.Error("GetDepositAddress() error cannot be nil")
		}
	}
}

func TestParseTime(t *testing.T) {
	t.Parallel()

	tm, err := parseTime("2019-11-21T02:08:34.87")
	if err != nil {
		t.Fatal(err)
	}

	if tm.Year() != 2019 ||
		tm.Month() != 11 ||
		tm.Day() != 21 ||
		tm.Hour() != 2 ||
		tm.Minute() != 8 ||
		tm.Second() != 34 {
		t.Error("invalid time values")
	}
}
