package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/vazha/gocryptotrader/common"
	"github.com/vazha/gocryptotrader/currency"
	"github.com/vazha/gocryptotrader/gctrpc"
	"github.com/urfave/cli"
)

var getInfoCommand = cli.Command{
	Name:   "getinfo",
	Usage:  "gets GoCryptoTrader info",
	Action: getInfo,
}

func getInfo(_ *cli.Context) error {
	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetInfo(context.Background(),
		&gctrpc.GetInfoRequest{},
	)

	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var getSubsystemsCommand = cli.Command{
	Name:   "getsubsystems",
	Usage:  "gets GoCryptoTrader subsystems and their status",
	Action: getSubsystems,
}

func getSubsystems(_ *cli.Context) error {
	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetSubsystems(context.Background(),
		&gctrpc.GetSubsystemsRequest{},
	)

	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var enableSubsystemCommand = cli.Command{
	Name:      "enablesubsystem",
	Usage:     "enables an engine subsystem",
	ArgsUsage: "<subsystem>",
	Action:    enableSubsystem,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "subsystem",
			Usage: "the subsystem to enable",
		},
	},
}

func enableSubsystem(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "enablesubsystem")
		return nil
	}

	var subsystemName string
	if c.IsSet("subsystem") {
		subsystemName = c.String("subsystem")
	} else {
		subsystemName = c.Args().First()
	}

	if subsystemName == "" {
		return errors.New("invalid subsystem supplied")
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.EnableSubsystem(context.Background(),
		&gctrpc.GenericSubsystemRequest{
			Subsystem: subsystemName,
		},
	)

	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var disableSubsystemCommand = cli.Command{
	Name:      "disablesubsystem",
	Usage:     "disables an engine subsystem",
	ArgsUsage: "<subsystem>",
	Action:    disableSubsystem,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "subsystem",
			Usage: "the subsystem to disable",
		},
	},
}

func disableSubsystem(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "disablesubsystem")
		return nil
	}

	var subsystemName string
	if c.IsSet("subsystem") {
		subsystemName = c.String("subsystem")
	} else {
		subsystemName = c.Args().First()
	}

	if subsystemName == "" {
		return errors.New("invalid subsystem supplied")
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.DisableSubsystem(context.Background(),
		&gctrpc.GenericSubsystemRequest{
			Subsystem: subsystemName,
		},
	)

	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var getRPCEndpointsCommand = cli.Command{
	Name:   "getrpcendpoints",
	Usage:  "gets GoCryptoTrader endpoints info",
	Action: getRPCEndpoints,
}

func getRPCEndpoints(_ *cli.Context) error {
	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetRPCEndpoints(context.Background(),
		&gctrpc.GetRPCEndpointsRequest{},
	)

	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var getCommunicationRelayersCommand = cli.Command{
	Name:   "getcommsrelayers",
	Usage:  "gets GoCryptoTrader communication relayers",
	Action: getCommunicationRelayers,
}

func getCommunicationRelayers(_ *cli.Context) error {
	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetCommunicationRelayers(context.Background(),
		&gctrpc.GetCommunicationRelayersRequest{},
	)

	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var getExchangesCommand = cli.Command{
	Name:      "getexchanges",
	Usage:     "gets a list of enabled or available exchanges",
	ArgsUsage: "<enabled>",
	Action:    getExchanges,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "enabled",
			Usage: "whether to list enabled exchanges or not",
		},
	},
}

func getExchanges(c *cli.Context) error {
	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	var enabledOnly bool
	if c.IsSet("enabled") {
		enabledOnly = c.Bool("enabled")
	}

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetExchanges(context.Background(),
		&gctrpc.GetExchangesRequest{
			Enabled: enabledOnly,
		},
	)

	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var enableExchangeCommand = cli.Command{
	Name:      "enableexchange",
	Usage:     "enables an exchange",
	ArgsUsage: "<exchange>",
	Action:    enableExchange,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to enable",
		},
	},
}

func enableExchange(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "enableexchange")
		return nil
	}

	var exchangeName string
	if c.IsSet("exchange") {
		exchangeName = c.String("exchange")
	} else {
		exchangeName = c.Args().First()
	}

	if !validExchange(exchangeName) {
		return errInvalidExchange
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.EnableExchange(context.Background(),
		&gctrpc.GenericExchangeNameRequest{
			Exchange: exchangeName,
		},
	)

	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var disableExchangeCommand = cli.Command{
	Name:      "disableexchange",
	Usage:     "disables an exchange",
	ArgsUsage: "<exchange>",
	Action:    disableExchange,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to disable",
		},
	},
}

func disableExchange(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "disableexchange")
		return nil
	}

	var exchangeName string
	if c.IsSet("exchange") {
		exchangeName = c.String("exchange")
	} else {
		exchangeName = c.Args().First()
	}

	if !validExchange(exchangeName) {
		return errInvalidExchange
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.DisableExchange(context.Background(),
		&gctrpc.GenericExchangeNameRequest{
			Exchange: exchangeName,
		},
	)

	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var getExchangeOTPCommand = cli.Command{
	Name:      "getexchangeotp",
	Usage:     "gets a specific exchange OTP code",
	ArgsUsage: "<exchange>",
	Action:    getExchangeOTPCode,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to get the OTP code for",
		},
	},
}

func getExchangeOTPCode(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "getexchangeotp")
		return nil
	}

	var exchangeName string
	if c.IsSet("exchange") {
		exchangeName = c.String("exchange")
	} else {
		exchangeName = c.Args().First()
	}

	if !validExchange(exchangeName) {
		return errInvalidExchange
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetExchangeOTPCode(context.Background(),
		&gctrpc.GenericExchangeNameRequest{
			Exchange: exchangeName,
		},
	)

	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var getExchangeOTPsCommand = cli.Command{
	Name:   "getexchangeotps",
	Usage:  "gets all exchange OTP codes",
	Action: getExchangeOTPCodes,
}

func getExchangeOTPCodes(c *cli.Context) error {
	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetExchangeOTPCodes(context.Background(),
		&gctrpc.GetExchangeOTPsRequest{})

	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var getExchangeInfoCommand = cli.Command{
	Name:      "getexchangeinfo",
	Usage:     "gets a specific exchanges info",
	ArgsUsage: "<exchange>",
	Action:    getExchangeInfo,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to get the info for",
		},
	},
}

func getExchangeInfo(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "getexchangeinfo")
		return nil
	}

	var exchangeName string
	if c.IsSet("exchange") {
		exchangeName = c.String("exchange")
	} else {
		exchangeName = c.Args().First()
	}

	if !validExchange(exchangeName) {
		return errInvalidExchange
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetExchangeInfo(context.Background(),
		&gctrpc.GenericExchangeNameRequest{
			Exchange: exchangeName,
		},
	)

	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var getTickerCommand = cli.Command{
	Name:      "getticker",
	Usage:     "gets the ticker for a specific currency pair and exchange",
	ArgsUsage: "<exchange> <pair> <asset>",
	Action:    getTicker,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to get the ticker for",
		},
		cli.StringFlag{
			Name:  "pair",
			Usage: "the currency pair to get the ticker for",
		},
		cli.StringFlag{
			Name:  "asset",
			Usage: "the asset type of the currency pair to get the ticker for",
		},
	},
}

func getTicker(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "getticker")
		return nil
	}

	var exchangeName string
	var currencyPair string
	var assetType string

	if c.IsSet("exchange") {
		exchangeName = c.String("exchange")
	} else {
		exchangeName = c.Args().First()
	}

	if !validExchange(exchangeName) {
		return errInvalidExchange
	}

	if c.IsSet("pair") {
		currencyPair = c.String("pair")
	} else {
		currencyPair = c.Args().Get(1)
	}

	if !validPair(currencyPair) {
		return errInvalidPair
	}

	if c.IsSet("asset") {
		assetType = c.String("asset")
	} else {
		assetType = c.Args().Get(2)
	}

	assetType = strings.ToLower(assetType)
	if !validAsset(assetType) {
		return errInvalidAsset
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	p := currency.NewPairDelimiter(currencyPair, pairDelimiter)
	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetTicker(context.Background(),
		&gctrpc.GetTickerRequest{
			Exchange: exchangeName,
			Pair: &gctrpc.CurrencyPair{
				Delimiter: p.Delimiter,
				Base:      p.Base.String(),
				Quote:     p.Quote.String(),
			},
			AssetType: assetType,
		},
	)

	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var getTickersCommand = cli.Command{
	Name:   "gettickers",
	Usage:  "gets all tickers for all enabled exchanges and currency pairs",
	Action: getTickers,
}

func getTickers(_ *cli.Context) error {
	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetTickers(context.Background(), &gctrpc.GetTickersRequest{})
	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var getOrderbookCommand = cli.Command{
	Name:      "getorderbook",
	Usage:     "gets the orderbook for a specific currency pair and exchange",
	ArgsUsage: "<exchange> <pair> <asset>",
	Action:    getOrderbook,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to get the orderbook for",
		},
		cli.StringFlag{
			Name:  "pair",
			Usage: "the currency pair to get the orderbook for",
		},
		cli.StringFlag{
			Name:  "asset",
			Usage: "the asset type of the currency pair to get the orderbook for",
		},
	},
}

func getOrderbook(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "getorderbook")
		return nil
	}

	var exchangeName string
	var currencyPair string
	var assetType string

	if c.IsSet("exchange") {
		exchangeName = c.String("exchange")
	} else {
		exchangeName = c.Args().First()
	}

	if !validExchange(exchangeName) {
		return errInvalidExchange
	}

	if c.IsSet("pair") {
		currencyPair = c.String("pair")
	} else {
		currencyPair = c.Args().Get(1)
	}

	if !validPair(currencyPair) {
		return errInvalidPair
	}

	if c.IsSet("asset") {
		assetType = c.String("asset")
	} else {
		assetType = c.Args().Get(2)
	}

	assetType = strings.ToLower(assetType)
	if !validAsset(assetType) {
		return errInvalidAsset
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	p := currency.NewPairDelimiter(currencyPair, pairDelimiter)
	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetOrderbook(context.Background(),
		&gctrpc.GetOrderbookRequest{
			Exchange: exchangeName,
			Pair: &gctrpc.CurrencyPair{
				Delimiter: p.Delimiter,
				Base:      p.Base.String(),
				Quote:     p.Quote.String(),
			},
			AssetType: assetType,
		},
	)

	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var getOrderbooksCommand = cli.Command{
	Name:   "getorderbooks",
	Usage:  "gets all orderbooks for all enabled exchanges and currency pairs",
	Action: getOrderbooks,
}

func getOrderbooks(_ *cli.Context) error {
	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetOrderbooks(context.Background(), &gctrpc.GetOrderbooksRequest{})
	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var getAccountInfoCommand = cli.Command{
	Name:      "getaccountinfo",
	Usage:     "gets the exchange account balance info",
	ArgsUsage: "<exchange>",
	Action:    getAccountInfo,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to get the account info for",
		},
	},
}

func getAccountInfo(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "getaccountinfo")
		return nil
	}

	var exchange string
	if c.IsSet("exchange") {
		exchange = c.String("exchange")
	} else {
		exchange = c.Args().First()
	}

	if !validExchange(exchange) {
		return errInvalidExchange
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetAccountInfo(context.Background(),
		&gctrpc.GetAccountInfoRequest{
			Exchange: exchange,
		},
	)
	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var getConfigCommand = cli.Command{
	Name:   "getconfig",
	Usage:  "gets the config",
	Action: getConfig,
}

func getConfig(_ *cli.Context) error {
	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetConfig(context.Background(), &gctrpc.GetConfigRequest{})
	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var getPortfolioCommand = cli.Command{
	Name:   "getportfolio",
	Usage:  "gets the portfolio",
	Action: getPortfolio,
}

func getPortfolio(_ *cli.Context) error {
	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetPortfolio(context.Background(), &gctrpc.GetPortfolioRequest{})
	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var getPortfolioSummaryCommand = cli.Command{
	Name:   "getportfoliosummary",
	Usage:  "gets the portfolio summary",
	Action: getPortfolioSummary,
}

func getPortfolioSummary(_ *cli.Context) error {
	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetPortfolioSummary(context.Background(), &gctrpc.GetPortfolioSummaryRequest{})
	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var addPortfolioAddressCommand = cli.Command{
	Name:      "addportfolioaddress",
	Usage:     "adds an address to the portfolio",
	ArgsUsage: "<address> <coin_type> <description> <balance>",
	Action:    addPortfolioAddress,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "address",
			Usage: "the address to add to the portfolio",
		},
		cli.StringFlag{
			Name:  "coin_type",
			Usage: "the coin type e.g ('BTC')",
		},
		cli.StringFlag{
			Name:  "description",
			Usage: "description of the address",
		},
		cli.Float64Flag{
			Name:  "balance",
			Usage: "balance of the address",
		},
	},
}

func addPortfolioAddress(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "addportfolioaddress")
		return nil
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	var address string
	var coinType string
	var description string
	var balance float64

	if c.IsSet("address") {
		address = c.String("address")
	} else {
		address = c.Args().First()
	}

	if c.IsSet("coin_type") {
		coinType = c.String("coin_type")
	} else {
		coinType = c.Args().Get(1)
	}

	if c.IsSet("description") {
		description = c.String("asset")
	} else {
		description = c.Args().Get(2)
	}

	if c.IsSet("balance") {
		balance = c.Float64("balance")
	} else {
		balance, _ = strconv.ParseFloat(c.Args().Get(3), 64)
	}

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.AddPortfolioAddress(context.Background(),
		&gctrpc.AddPortfolioAddressRequest{
			Address:     address,
			CoinType:    coinType,
			Description: description,
			Balance:     balance,
		},
	)

	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var removePortfolioAddressCommand = cli.Command{
	Name:      "removeportfolioaddress",
	Usage:     "removes an address from the portfolio",
	ArgsUsage: "<address> <coin_type> <description>",
	Action:    removePortfolioAddress,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "address",
			Usage: "the address to add to the portfolio",
		},
		cli.StringFlag{
			Name:  "coin_type",
			Usage: "the coin type e.g ('BTC')",
		},
		cli.StringFlag{
			Name:  "description",
			Usage: "description of the address",
		},
	},
}

func removePortfolioAddress(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "removeportfolioaddress")
		return nil
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	var address string
	var coinType string
	var description string

	if c.IsSet("address") {
		address = c.String("address")
	} else {
		address = c.Args().First()
	}

	if c.IsSet("coin_type") {
		coinType = c.String("coin_type")
	} else {
		coinType = c.Args().Get(1)
	}

	if c.IsSet("description") {
		description = c.String("asset")
	} else {
		description = c.Args().Get(2)
	}

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.RemovePortfolioAddress(context.Background(),
		&gctrpc.RemovePortfolioAddressRequest{
			Address:     address,
			CoinType:    coinType,
			Description: description,
		},
	)

	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var getForexProvidersCommand = cli.Command{
	Name:   "getforexproviders",
	Usage:  "gets the available forex providers",
	Action: getForexProviders,
}

func getForexProviders(_ *cli.Context) error {
	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetForexProviders(context.Background(), &gctrpc.GetForexProvidersRequest{})
	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var getForexRatesCommand = cli.Command{
	Name:   "getforexrates",
	Usage:  "gets forex rates",
	Action: getForexRates,
}

func getForexRates(_ *cli.Context) error {
	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetForexRates(context.Background(), &gctrpc.GetForexRatesRequest{})
	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var getOrdersCommand = cli.Command{
	Name:      "getorders",
	Usage:     "gets the open orders",
	ArgsUsage: "<exchange> <asset_type> <pair>",
	Action:    getOrders,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to get orders for",
		},
		cli.StringFlag{
			Name:  "asset_type",
			Usage: "the asset type to get orders for",
		},
		cli.StringFlag{
			Name:  "pair",
			Usage: "the currency pair to get orders for",
		},
	},
}

func getOrders(c *cli.Context) error {
	var exchangeName string
	var assetType string
	var currencyPair string

	if c.IsSet("exchange") {
		exchangeName = c.String("exchange")
	} else {
		exchangeName = c.Args().First()
	}

	if !validExchange(exchangeName) {
		return errInvalidExchange
	}

	if c.IsSet("asset_type") {
		assetType = c.String("asset_type")
	} else {
		assetType = c.Args().Get(1)
	}

	assetType = strings.ToLower(assetType)
	if !validAsset(assetType) {
		return errInvalidAsset
	}

	if c.IsSet("pair") {
		currencyPair = c.String("pair")
	} else {
		currencyPair = c.Args().Get(2)
	}

	if !validPair(currencyPair) {
		return errInvalidPair
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	p := currency.NewPairDelimiter(currencyPair, pairDelimiter)
	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetOrders(context.Background(), &gctrpc.GetOrdersRequest{
		Exchange:  exchangeName,
		AssetType: assetType,
		Pair: &gctrpc.CurrencyPair{
			Delimiter: p.Delimiter,
			Base:      p.Base.String(),
			Quote:     p.Quote.String(),
		},
	})
	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var getOrderCommand = cli.Command{
	Name:      "getorder",
	Usage:     "gets the specified order info",
	ArgsUsage: "<exchange> <order_id>",
	Action:    getOrder,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to get the order for",
		},
		cli.StringFlag{
			Name:  "order_id",
			Usage: "the order id to retrieve",
		},
	},
}

func getOrder(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "getorder")
		return nil
	}

	var exchangeName string
	var orderID string

	if c.IsSet("exchange") {
		exchangeName = c.String("exchange")
	} else {
		exchangeName = c.Args().First()
	}

	if !validExchange(exchangeName) {
		return errInvalidExchange
	}

	if c.IsSet("order_id") {
		orderID = c.String("order_id")
	} else {
		orderID = c.Args().Get(1)
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetOrder(context.Background(), &gctrpc.GetOrderRequest{
		Exchange: exchangeName,
		OrderId:  orderID,
	})
	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var submitOrderCommand = cli.Command{
	Name:      "submitorder",
	Usage:     "submit order submits an exchange order",
	ArgsUsage: "<exchange> <pair> <side> <order_type> <amount> <price> <client_id>",
	Action:    submitOrder,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to submit the order for",
		},
		cli.StringFlag{
			Name:  "pair",
			Usage: "the currency pair",
		},
		cli.StringFlag{
			Name:  "side",
			Usage: "the order side to use (BUY OR SELL)",
		},
		cli.StringFlag{
			Name:  "order_type",
			Usage: "the order type (MARKET OR LIMIT)",
		},
		cli.Float64Flag{
			Name:  "amount",
			Usage: "the amount for the order",
		},
		cli.Float64Flag{
			Name:  "price",
			Usage: "the price for the order",
		},
		cli.StringFlag{
			Name:  "client_id",
			Usage: "the optional client order ID",
		},
	},
}

func submitOrder(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "submitorder")
		return nil
	}

	var exchangeName string
	var currencyPair string
	var orderSide string
	var orderType string
	var amount float64
	var price float64
	var clientID string

	if c.IsSet("exchange") {
		exchangeName = c.String("exchange")
	} else {
		exchangeName = c.Args().First()
	}

	if !validExchange(exchangeName) {
		return errInvalidExchange
	}

	if c.IsSet("pair") {
		currencyPair = c.String("pair")
	} else {
		currencyPair = c.Args().Get(1)
	}

	if !validPair(currencyPair) {
		return errInvalidPair
	}

	if c.IsSet("side") {
		orderSide = c.String("side")
	} else {
		orderSide = c.Args().Get(2)
	}

	if c.IsSet("order_type") {
		orderType = c.String("order_type")
	} else {
		orderType = c.Args().Get(3)
	}

	if c.IsSet("amount") {
		amount = c.Float64("amount")
	} else {
		amount, _ = strconv.ParseFloat(c.Args().Get(4), 64)
	}

	if c.IsSet("price") {
		price = c.Float64("price")
	} else {
		price, _ = strconv.ParseFloat(c.Args().Get(5), 64)
	}

	if c.IsSet("client_id") {
		clientID = c.String("client_id")
	} else {
		clientID = c.Args().Get(6)
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	p := currency.NewPairDelimiter(currencyPair, pairDelimiter)
	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.SubmitOrder(context.Background(), &gctrpc.SubmitOrderRequest{
		Exchange: exchangeName,
		Pair: &gctrpc.CurrencyPair{
			Delimiter: p.Delimiter,
			Base:      p.Base.String(),
			Quote:     p.Quote.String(),
		},
		Side:      orderSide,
		OrderType: orderType,
		Amount:    amount,
		Price:     price,
		ClientId:  clientID,
	})
	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var simulateOrderCommand = cli.Command{
	Name:      "simulateorder",
	Usage:     "simulate order simulates an exchange order",
	ArgsUsage: "<exchange> <pair> <side> <amount>",
	Action:    simulateOrder,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to simulate the order for",
		},
		cli.StringFlag{
			Name:  "pair",
			Usage: "the currency pair",
		},
		cli.StringFlag{
			Name:  "side",
			Usage: "the order side to use (BUY OR SELL)",
		},
		cli.Float64Flag{
			Name:  "amount",
			Usage: "the amount for the order",
		},
	},
}

func simulateOrder(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "simulateorder")
		return nil
	}

	var exchangeName string
	var currencyPair string
	var orderSide string
	var amount float64

	if c.IsSet("exchange") {
		exchangeName = c.String("exchange")
	} else {
		exchangeName = c.Args().First()
	}

	if !validExchange(exchangeName) {
		return errInvalidExchange
	}

	if c.IsSet("pair") {
		currencyPair = c.String("pair")
	} else {
		currencyPair = c.Args().Get(1)
	}

	if !validPair(currencyPair) {
		return errInvalidPair
	}

	if c.IsSet("side") {
		orderSide = c.String("side")
	} else {
		orderSide = c.Args().Get(2)
	}

	if c.IsSet("amount") {
		amount = c.Float64("amount")
	} else {
		amount, _ = strconv.ParseFloat(c.Args().Get(3), 64)
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	p := currency.NewPairDelimiter(currencyPair, pairDelimiter)
	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.SimulateOrder(context.Background(), &gctrpc.SimulateOrderRequest{
		Exchange: exchangeName,
		Pair: &gctrpc.CurrencyPair{
			Delimiter: p.Delimiter,
			Base:      p.Base.String(),
			Quote:     p.Quote.String(),
		},
		Side:   orderSide,
		Amount: amount,
	})
	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var whaleBombCommand = cli.Command{
	Name:      "whalebomb",
	Usage:     "whale bomb finds the amount required to reach a price target",
	ArgsUsage: "<exchange> <pair> <side> <price>",
	Action:    whaleBomb,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to whale bomb",
		},
		cli.StringFlag{
			Name:  "pair",
			Usage: "the currency pair",
		},
		cli.StringFlag{
			Name:  "side",
			Usage: "the order side to use (BUY OR SELL)",
		},
		cli.Float64Flag{
			Name:  "price",
			Usage: "the price target",
		},
	},
}

func whaleBomb(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "whalebomb")
		return nil
	}

	var exchangeName string
	var currencyPair string
	var orderSide string
	var price float64

	if c.IsSet("exchange") {
		exchangeName = c.String("exchange")
	} else {
		exchangeName = c.Args().First()
	}

	if !validExchange(exchangeName) {
		return errInvalidExchange
	}

	if c.IsSet("pair") {
		currencyPair = c.String("pair")
	} else {
		currencyPair = c.Args().Get(1)
	}

	if !validPair(currencyPair) {
		return errInvalidPair
	}

	if c.IsSet("side") {
		orderSide = c.String("side")
	} else {
		orderSide = c.Args().Get(2)
	}

	if c.IsSet("price") {
		price = c.Float64("price")
	} else {
		price, _ = strconv.ParseFloat(c.Args().Get(3), 64)
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	p := currency.NewPairDelimiter(currencyPair, pairDelimiter)
	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.WhaleBomb(context.Background(), &gctrpc.WhaleBombRequest{
		Exchange: exchangeName,
		Pair: &gctrpc.CurrencyPair{
			Delimiter: p.Delimiter,
			Base:      p.Base.String(),
			Quote:     p.Quote.String(),
		},
		Side:        orderSide,
		PriceTarget: price,
	})
	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var cancelOrderCommand = cli.Command{
	Name:      "cancelorder",
	Usage:     "cancel order cancels an exchange order",
	ArgsUsage: "<exchange> <account_id> <order_id> <pair> <asset_type> <wallet_address> <side>",
	Action:    cancelOrder,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to cancel the order for",
		},
		cli.StringFlag{
			Name:  "account_id",
			Usage: "the account id",
		},
		cli.StringFlag{
			Name:  "order_id",
			Usage: "the order id",
		},
		cli.StringFlag{
			Name:  "pair",
			Usage: "the currency pair to cancel the order for",
		},
		cli.StringFlag{
			Name:  "asset_type",
			Usage: "the asset type",
		},
		cli.Float64Flag{
			Name:  "wallet_address",
			Usage: "the wallet address",
		},
		cli.StringFlag{
			Name:  "side",
			Usage: "the order side",
		},
	},
}

func cancelOrder(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "cancelorder")
		return nil
	}

	var exchangeName string
	var accountID string
	var orderID string
	var currencyPair string
	var assetType string
	var walletAddress string
	var orderSide string

	if c.IsSet("exchange") {
		exchangeName = c.String("exchange")
	} else {
		exchangeName = c.Args().First()
	}

	if !validExchange(exchangeName) {
		return errInvalidExchange
	}

	if c.IsSet("order_id") {
		orderID = c.String("order_id")
	} else {
		orderID = c.Args().Get(2)
	}

	if c.IsSet("account_id") {
		accountID = c.String("account_id")
	}

	if c.IsSet("pair") {
		currencyPair = c.String("pair")
	}

	if c.IsSet("asset_type") {
		assetType = c.String("asset_type")
	}

	assetType = strings.ToLower(assetType)
	if !validAsset(assetType) {
		return errInvalidAsset
	}

	if c.IsSet("wallet_address") {
		walletAddress = c.String("wallet_address")
	}

	if c.IsSet("order_side") {
		orderSide = c.String("order_side")
	}

	var p currency.Pair
	if len(currencyPair) > 0 {
		if !validPair(currencyPair) {
			return errInvalidPair
		}
		p = currency.NewPairDelimiter(currencyPair, pairDelimiter)
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.CancelOrder(context.Background(), &gctrpc.CancelOrderRequest{
		Exchange:  exchangeName,
		AccountId: accountID,
		OrderId:   orderID,
		Pair: &gctrpc.CurrencyPair{
			Delimiter: p.Delimiter,
			Base:      p.Base.String(),
			Quote:     p.Quote.String(),
		},
		AssetType:     assetType,
		WalletAddress: walletAddress,
		Side:          orderSide,
	})
	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var cancelAllOrdersCommand = cli.Command{
	Name:      "cancelallorders",
	Usage:     "cancels all orders (all or by exchange name)",
	ArgsUsage: "<exchange>",
	Action:    cancelAllOrders,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to cancel all orders on",
		},
	},
}

func cancelAllOrders(c *cli.Context) error {
	var exchangeName string
	if c.IsSet("exchange") {
		exchangeName = c.String("exchange")
	} else {
		exchangeName = c.Args().First()
	}

	// exchange name is an optional param
	if exchangeName != "" {
		if !validExchange(exchangeName) {
			return errInvalidExchange
		}
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.CancelAllOrders(context.Background(), &gctrpc.CancelAllOrdersRequest{
		Exchange: exchangeName,
	})
	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var getEventsCommand = cli.Command{
	Name:   "getevents",
	Usage:  "gets all events",
	Action: getEvents,
}

func getEvents(_ *cli.Context) error {
	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetEvents(context.Background(), &gctrpc.GetEventsRequest{})
	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var addEventCommand = cli.Command{
	Name:      "addevent",
	Usage:     "adds an event",
	ArgsUsage: "<exchange> <item> <condition> <price> <check_bids> <check_bids_and_asks> <orderbook_amount> <pair> <asset_type> <action>",
	Action:    addEvent,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to add an event for",
		},
		cli.StringFlag{
			Name:  "item",
			Usage: "the item to trigger the event",
		},
		cli.StringFlag{
			Name:  "condition",
			Usage: "the condition for the event",
		},
		cli.Float64Flag{
			Name:  "price",
			Usage: "the price to trigger the event",
		},
		cli.BoolFlag{
			Name:  "check_bids",
			Usage: "whether to check the bids (if false, asks will be used)",
		},
		cli.BoolFlag{
			Name:  "check_bids_and_asks",
			Usage: "the wallet address",
		},
		cli.Float64Flag{
			Name:  "orderbook_amount",
			Usage: "the orderbook amount to trigger the event",
		},
		cli.StringFlag{
			Name:  "pair",
			Usage: "the currency pair",
		},
		cli.StringFlag{
			Name:  "asset_type",
			Usage: "the asset type",
		},
		cli.StringFlag{
			Name:  "action",
			Usage: "the action for the event to perform upon trigger",
		},
	},
}

func addEvent(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "addevent")
		return nil
	}

	var exchangeName string
	var item string
	var condition string
	var price float64
	var checkBids bool
	var checkBidsAndAsks bool
	var orderbookAmount float64
	var currencyPair string
	var assetType string
	var action string

	if c.IsSet("exchange") {
		exchangeName = c.String("exchange")
	} else {
		return fmt.Errorf("exchange name is required")
	}

	if c.IsSet("item") {
		item = c.String("item")
	} else {
		return fmt.Errorf("item is required")
	}

	if c.IsSet("condition") {
		condition = c.String("condition")
	} else {
		return fmt.Errorf("condition is required")
	}

	if c.IsSet("price") {
		price = c.Float64("price")
	}

	if c.IsSet("check_bids") {
		checkBids = c.Bool("check_bids")
	}

	if c.IsSet("check_bids_and_asks") {
		checkBids = c.Bool("check_bids_and_asks")
	}

	if c.IsSet("orderbook_amount") {
		orderbookAmount = c.Float64("orderbook_amount")
	}

	if c.IsSet("pair") {
		currencyPair = c.String("pair")
	} else {
		return fmt.Errorf("currency pair is required")
	}

	if c.IsSet("asset_type") {
		assetType = c.String("asset_type")
	}

	assetType = strings.ToLower(assetType)
	if !validAsset(assetType) {
		return errInvalidAsset
	}

	if c.IsSet("action") {
		action = c.String("action")
	} else {
		return fmt.Errorf("action is required")
	}

	if !validPair(currencyPair) {
		return errInvalidPair
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	p := currency.NewPairDelimiter(currencyPair, pairDelimiter)
	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.AddEvent(context.Background(), &gctrpc.AddEventRequest{
		Exchange: exchangeName,
		Item:     item,
		ConditionParams: &gctrpc.ConditionParams{
			Condition:        condition,
			Price:            price,
			CheckBids:        checkBids,
			CheckBidsAndAsks: checkBidsAndAsks,
			OrderbookAmount:  orderbookAmount,
		},
		Pair: &gctrpc.CurrencyPair{
			Delimiter: p.Delimiter,
			Base:      p.Base.String(),
			Quote:     p.Quote.String(),
		},
		AssetType: assetType,
		Action:    action,
	})
	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var removeEventCommand = cli.Command{
	Name:      "removeevent",
	Usage:     "removes an event",
	ArgsUsage: "<event_id>",
	Action:    removeEvent,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "event_id",
			Usage: "the event id to remove",
		},
	},
}

func removeEvent(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "removeevent")
		return nil
	}

	var eventID int64
	if c.IsSet("event_id") {
		eventID = c.Int64("event_id")
	} else {
		evtID, err := strconv.Atoi(c.Args().Get(0))
		if err != nil {
			return fmt.Errorf("unable to strconv input to int. Err: %s", err)
		}
		eventID = int64(evtID)
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.RemoveEvent(context.Background(),
		&gctrpc.RemoveEventRequest{Id: eventID})
	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var getCryptocurrencyDepositAddressesCommand = cli.Command{
	Name:      "getcryptocurrencydepositaddresses",
	Usage:     "gets the cryptocurrency deposit addresses for an exchange",
	ArgsUsage: "<exchange>",
	Action:    getCryptocurrencyDepositAddresses,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to get the cryptocurrency deposit addresses for",
		},
	},
}

func getCryptocurrencyDepositAddresses(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "getcryptocurrencydepositaddresses")
		return nil
	}

	var exchangeName string
	if c.IsSet("exchange") {
		exchangeName = c.String("exchange")
	} else {
		exchangeName = c.Args().First()
	}

	if !validExchange(exchangeName) {
		return errInvalidExchange
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetCryptocurrencyDepositAddresses(context.Background(),
		&gctrpc.GetCryptocurrencyDepositAddressesRequest{Exchange: exchangeName})
	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var getCryptocurrencyDepositAddressCommand = cli.Command{
	Name:      "getcryptocurrencydepositaddress",
	Usage:     "gets the cryptocurrency deposit address for an exchange and cryptocurrency",
	ArgsUsage: "<exchange> <cryptocurrency>",
	Action:    getCryptocurrencyDepositAddress,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to get the cryptocurrency deposit address for",
		},
		cli.StringFlag{
			Name:  "cryptocurrency",
			Usage: "the cryptocurrency to get the deposit address for",
		},
	},
}

func getCryptocurrencyDepositAddress(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "getcryptocurrencydepositaddresses")
		return nil
	}

	var exchangeName string
	var cryptocurrency string

	if c.IsSet("exchange") {
		exchangeName = c.String("exchange")
	} else {
		exchangeName = c.Args().First()
	}

	if !validExchange(exchangeName) {
		return errInvalidExchange
	}

	if c.IsSet("cryptocurrency") {
		cryptocurrency = c.String("cryptocurrency")
	} else {
		cryptocurrency = c.Args().Get(1)
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetCryptocurrencyDepositAddress(context.Background(),
		&gctrpc.GetCryptocurrencyDepositAddressRequest{
			Exchange:       exchangeName,
			Cryptocurrency: cryptocurrency,
		},
	)
	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}

var withdrawCryptocurrencyFundsCommand = cli.Command{
	Name:      "withdrawcryptocurrencyfunds",
	Usage:     "withdraws cryptocurrency funds from the desired exchange",
	ArgsUsage: "<exchange> <cryptocurrency>",
	Action:    withdrawCryptocurrencyFunds,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to withdraw from",
		},
		cli.StringFlag{
			Name:  "cryptocurrency",
			Usage: "the cryptocurrency to withdraw funds from",
		},
	},
}

func withdrawCryptocurrencyFunds(_ *cli.Context) error {
	return common.ErrNotYetImplemented
}

var withdrawFiatFundsCommand = cli.Command{
	Name:      "withdrawfiatfunds",
	Usage:     "withdraws fiat funds from the desired exchange",
	ArgsUsage: "<exchange> <fiat_currency>",
	Action:    withdrawFiatFunds,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to withdraw from",
		},
		cli.StringFlag{
			Name:  "fiat_currency",
			Usage: "the fiat currency to withdraw funds from",
		},
	},
}

func withdrawFiatFunds(_ *cli.Context) error {
	return common.ErrNotYetImplemented
}

var getLoggerDetailsCommand = cli.Command{
	Name:      "getloggerdetails",
	Usage:     "gets an individual loggers details",
	ArgsUsage: "<logger>",
	Action:    getLoggerDetails,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "logger",
			Usage: "logger to get level details of",
		},
	},
}

func getLoggerDetails(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "getloggerdetails")
		return nil
	}

	var logger string
	if c.IsSet("logger") {
		logger = c.String("logger")
	} else {
		logger = c.Args().First()
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)

	result, err := client.GetLoggerDetails(context.Background(),
		&gctrpc.GetLoggerDetailsRequest{
			Logger: logger,
		},
	)
	if err != nil {
		return err
	}
	jsonOutput(result)
	return nil
}

var setLoggerDetailsCommand = cli.Command{
	Name:      "setloggerdetails",
	Usage:     "sets an individual loggers details",
	ArgsUsage: "<logger> <flags>",
	Action:    setLoggerDetails,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "logger",
			Usage: "logger to get level details of",
		},
		cli.StringFlag{
			Name:  "flags",
			Usage: "pipe separated value of loggers e.g INFO|WARN",
		},
	},
}

func setLoggerDetails(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "setloggerdetails")
		return nil
	}

	var logger string
	var level string

	if c.IsSet("logger") {
		logger = c.String("logger")
	} else {
		logger = c.Args().First()
	}

	if c.IsSet("level") {
		level = c.String("level")
	} else {
		level = c.Args().Get(1)
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)

	result, err := client.SetLoggerDetails(context.Background(),
		&gctrpc.SetLoggerDetailsRequest{
			Logger: logger,
			Level:  level,
		},
	)
	if err != nil {
		return err
	}
	jsonOutput(result)
	return nil
}

var getExchangePairsCommand = cli.Command{
	Name:      "getexchangepairs",
	Usage:     "gets an exchanges supported currency pairs (available and enabled) plus asset types",
	ArgsUsage: "<exchange> <asset>",
	Action:    getExchangePairs,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to list of the currency pairs of",
		},
		cli.StringFlag{
			Name:  "asset",
			Usage: "the asset type to filter by",
		},
	},
}

func getExchangePairs(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "getexchangepairs")
		return nil
	}

	var exchange string
	var asset string

	if c.IsSet("exchange") {
		exchange = c.String("exchange")
	} else {
		exchange = c.Args().First()
	}

	if !validExchange(exchange) {
		return errInvalidExchange
	}

	if c.IsSet("asset") {
		asset = c.String("asset")
	} else {
		asset = c.Args().Get(1)
	}

	asset = strings.ToLower(asset)
	if !validAsset(asset) {
		return errInvalidAsset
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetExchangePairs(context.Background(),
		&gctrpc.GetExchangePairsRequest{
			Exchange: exchange,
			Asset:    asset,
		},
	)
	if err != nil {
		return err
	}
	jsonOutput(result)
	return nil
}

var enableExchangePairCommand = cli.Command{
	Name:      "enableexchangepair",
	Usage:     "enables an exchange currency pair",
	ArgsUsage: "<exchange> <pair> <asset>",
	Action:    enableExchangePair,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to enable the currency pair for",
		},
		cli.StringFlag{
			Name:  "pair",
			Usage: "the currency pair to enable",
		},
		cli.StringFlag{
			Name:  "asset",
			Usage: "the asset type to enable the currency pair for",
		},
	},
}

func enableExchangePair(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "enableexchangepair")
		return nil
	}

	var exchange string
	var pair string
	var asset string

	if c.IsSet("exchange") {
		exchange = c.String("exchange")
	} else {
		exchange = c.Args().First()
	}

	if !validExchange(exchange) {
		return errInvalidExchange
	}

	if c.IsSet("pair") {
		pair = c.String("pair")
	} else {
		pair = c.Args().Get(1)
	}

	if !validPair(pair) {
		return errInvalidPair
	}

	if c.IsSet("asset") {
		asset = c.String("asset")
	} else {
		asset = c.Args().Get(2)
	}

	asset = strings.ToLower(asset)
	if !validAsset(asset) {
		return errInvalidAsset
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	p := currency.NewPairDelimiter(pair, pairDelimiter)
	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.EnableExchangePair(context.Background(),
		&gctrpc.ExchangePairRequest{
			Exchange: exchange,
			Pair: &gctrpc.CurrencyPair{
				Delimiter: p.Delimiter,
				Base:      p.Base.String(),
				Quote:     p.Quote.String(),
			},
			AssetType: asset,
		},
	)
	if err != nil {
		return err
	}
	jsonOutput(result)
	return nil
}

var disableExchangePairCommand = cli.Command{
	Name:      "disableexchangepair",
	Usage:     "disables a previously enabled exchange currency pair",
	ArgsUsage: "<exchange> <pair> <asset>",
	Action:    disableExchangePair,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to disable the currency pair for",
		},
		cli.StringFlag{
			Name:  "pair",
			Usage: "the currency pair to disable",
		},
		cli.StringFlag{
			Name:  "asset",
			Usage: "the asset type to disable the currency pair for",
		},
	},
}

func disableExchangePair(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "disableexchangepair")
		return nil
	}

	var exchange string
	var pair string
	var asset string

	if c.IsSet("exchange") {
		exchange = c.String("exchange")
	} else {
		exchange = c.Args().First()
	}

	if !validExchange(exchange) {
		return errInvalidExchange
	}

	if c.IsSet("pair") {
		pair = c.String("pair")
	} else {
		pair = c.Args().Get(1)
	}

	if !validPair(pair) {
		return errInvalidPair
	}

	if c.IsSet("asset") {
		asset = c.String("asset")
	} else {
		asset = c.Args().Get(2)
	}

	asset = strings.ToLower(asset)
	if !validAsset(asset) {
		return errInvalidAsset
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	p := currency.NewPairDelimiter(pair, pairDelimiter)
	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.DisableExchangePair(context.Background(),
		&gctrpc.ExchangePairRequest{
			Exchange: exchange,
			Pair: &gctrpc.CurrencyPair{
				Delimiter: p.Delimiter,
				Base:      p.Base.String(),
				Quote:     p.Quote.String(),
			},
			AssetType: asset,
		},
	)
	if err != nil {
		return err
	}
	jsonOutput(result)
	return nil
}

var getOrderbookStreamCommand = cli.Command{
	Name:      "getorderbookstream",
	Usage:     "gets the orderbook stream for a specific currency pair and exchange",
	ArgsUsage: "<exchange> <currencyPair> <asset>",
	Action:    getOrderbookStream,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to get the orderbook from",
		},
		cli.StringFlag{
			Name:  "pair",
			Usage: "currency pair",
		},
		cli.StringFlag{
			Name:  "asset",
			Usage: "the asset type of the currency pair",
		},
	},
}

func getOrderbookStream(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "getorderbookstream")
		return nil
	}

	var exchangeName string
	var pair string
	var assetType string

	if c.IsSet("exchange") {
		exchangeName = c.String("exchange")
	} else {
		exchangeName = c.Args().First()
	}

	if !validExchange(exchangeName) {
		return errInvalidExchange
	}

	if c.IsSet("pair") {
		pair = c.String("pair")
	} else {
		pair = c.Args().Get(1)
	}

	if !validPair(pair) {
		return errInvalidPair
	}

	if c.IsSet("asset") {
		assetType = c.String("asset")
	} else {
		assetType = c.Args().Get(2)
	}

	assetType = strings.ToLower(assetType)

	if !validAsset(assetType) {
		return errInvalidAsset
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	p := currency.NewPairDelimiter(pair, pairDelimiter)

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetOrderbookStream(context.Background(),
		&gctrpc.GetOrderbookStreamRequest{
			Exchange: exchangeName,
			Pair: &gctrpc.CurrencyPair{
				Base:      p.Base.String(),
				Quote:     p.Quote.String(),
				Delimiter: p.Delimiter,
			},
			AssetType: assetType,
		},
	)

	if err != nil {
		return err
	}

	for {
		resp, err := result.Recv()
		if err != nil {
			return err
		}

		err = clearScreen()
		if err != nil {
			return err
		}

		fmt.Printf("Orderbook stream for %s %s:\n\n", exchangeName,
			resp.Pair.String())
		fmt.Println("\t\tBids\t\t\t\tAsks")
		fmt.Println()

		bidLen := len(resp.Bids) - 1
		askLen := len(resp.Asks) - 1

		var maxLen int
		if bidLen >= askLen {
			maxLen = bidLen
		} else {
			maxLen = askLen
		}

		for i := 0; i < maxLen; i++ {
			var bidAmount, bidPrice float64
			if i <= bidLen {
				bidAmount = resp.Bids[i].Amount
				bidPrice = resp.Bids[i].Price
			}

			var askAmount, askPrice float64
			if i <= askLen {
				askAmount = resp.Asks[i].Amount
				askPrice = resp.Asks[i].Price
			}

			fmt.Printf("%f %s @ %f %s\t\t%f %s @ %f %s\n",
				bidAmount,
				resp.Pair.Base,
				bidPrice,
				resp.Pair.Quote,
				askAmount,
				resp.Pair.Base,
				askPrice,
				resp.Pair.Quote)

			if i >= 49 {
				// limits orderbook display output
				break
			}
		}
	}
}

var getExchangeOrderbookStreamCommand = cli.Command{
	Name:      "getexchangeorderbookstream",
	Usage:     "gets a stream for all orderbooks associated with an exchange",
	ArgsUsage: "<exchange>",
	Action:    getExchangeOrderbookStream,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to get the orderbook from",
		},
	},
}

func getExchangeOrderbookStream(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "getexchangeorderbookstream")
		return nil
	}

	var exchangeName string
	if c.IsSet("exchange") {
		exchangeName = c.String("exchange")
	} else {
		exchangeName = c.Args().First()
	}

	if !validExchange(exchangeName) {
		return errInvalidExchange
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetExchangeOrderbookStream(context.Background(),
		&gctrpc.GetExchangeOrderbookStreamRequest{
			Exchange: exchangeName,
		})

	if err != nil {
		return err
	}

	for {
		resp, err := result.Recv()
		if err != nil {
			return err
		}

		err = clearScreen()
		if err != nil {
			return err
		}

		fmt.Printf("Orderbook streamed for %s %s",
			exchangeName,
			resp.Pair.String())
	}
}

var getTickerStreamCommand = cli.Command{
	Name:      "gettickerstream",
	Usage:     "gets the ticker stream for a specific currency pair and exchange",
	ArgsUsage: "<exchange> <currencyPair> <asset>",
	Action:    getTickerStream,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to get the ticker from",
		},
		cli.StringFlag{
			Name:  "pair",
			Usage: "currency pair",
		},
		cli.StringFlag{
			Name:  "asset",
			Usage: "the asset type of the currency pair",
		},
	},
}

func getTickerStream(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "gettickerstream")
		return nil
	}

	var exchangeName string
	var pair string
	var assetType string

	if c.IsSet("exchange") {
		exchangeName = c.String("exchange")
	} else {
		exchangeName = c.Args().First()
	}

	if !validExchange(exchangeName) {
		return errInvalidExchange
	}

	if c.IsSet("pair") {
		pair = c.String("pair")
	} else {
		pair = c.Args().Get(1)
	}

	if !validPair(pair) {
		return errInvalidPair
	}

	if c.IsSet("asset") {
		assetType = c.String("asset")
	} else {
		assetType = c.Args().Get(2)
	}

	assetType = strings.ToLower(assetType)

	if !validAsset(assetType) {
		return errInvalidAsset
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	p := currency.NewPairDelimiter(pair, pairDelimiter)

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetTickerStream(context.Background(),
		&gctrpc.GetTickerStreamRequest{
			Exchange: exchangeName,
			Pair: &gctrpc.CurrencyPair{
				Base:      p.Base.String(),
				Quote:     p.Quote.String(),
				Delimiter: p.Delimiter,
			},
			AssetType: assetType,
		},
	)

	if err != nil {
		return err
	}

	for {
		resp, err := result.Recv()
		if err != nil {
			return err
		}

		err = clearScreen()
		if err != nil {
			return err
		}

		fmt.Printf("Ticker stream for %s %s:\n", exchangeName,
			resp.Pair.String())
		fmt.Println()

		fmt.Printf("LAST: %f\n HIGH: %f\n LOW: %f\n BID: %f\n ASK: %f\n VOLUME: %f\n PRICEATH: %f\n LASTUPDATED: %d\n",
			resp.Last,
			resp.High,
			resp.Low,
			resp.Bid,
			resp.Ask,
			resp.Volume,
			resp.PriceAth,
			resp.LastUpdated)
	}
}

var getExchangeTickerStreamCommand = cli.Command{
	Name:      "getexchangetickerstream",
	Usage:     "gets a stream for all tickers associated with an exchange",
	ArgsUsage: "<exchange>",
	Action:    getExchangeTickerStream,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exchange",
			Usage: "the exchange to get the ticker from",
		},
	},
}

func getExchangeTickerStream(c *cli.Context) error {
	if c.NArg() == 0 && c.NumFlags() == 0 {
		cli.ShowCommandHelp(c, "getexchangetickerstream")
		return nil
	}

	var exchangeName string
	if c.IsSet("exchange") {
		exchangeName = c.String("exchange")
	} else {
		exchangeName = c.Args().First()
	}

	if !validExchange(exchangeName) {
		return errInvalidExchange
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)
	result, err := client.GetExchangeTickerStream(context.Background(),
		&gctrpc.GetExchangeTickerStreamRequest{
			Exchange: exchangeName,
		})

	if err != nil {
		return err
	}

	for {
		resp, err := result.Recv()
		if err != nil {
			return err
		}

		fmt.Printf("Ticker stream for %s %s:\n",
			exchangeName,
			resp.Pair.String())

		fmt.Printf("LAST: %f HIGH: %f LOW: %f BID: %f ASK: %f VOLUME: %f PRICEATH: %f LASTUPDATED: %d\n",
			resp.Last,
			resp.High,
			resp.Low,
			resp.Bid,
			resp.Ask,
			resp.Volume,
			resp.PriceAth,
			resp.LastUpdated)
	}
}

func clearScreen() error {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		return cmd.Run()
	default:
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		return cmd.Run()
	}
}

const timeFormat = "2006-01-02 15:04:05"

var startTime, endTime, order string
var limit int

var getAuditEventCommand = cli.Command{
	Name:      "getauditevent",
	Usage:     "gets audit events matching query parameters",
	ArgsUsage: "<starttime> <endtime> <orderby> <limit>",
	Action:    getAuditEvent,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:        "start, s",
			Usage:       "start date to search",
			Value:       time.Now().Add(-time.Hour).Format(timeFormat),
			Destination: &startTime,
		},
		cli.StringFlag{
			Name:        "end, e",
			Usage:       "end time to search",
			Value:       time.Now().Format(timeFormat),
			Destination: &endTime,
		},
		cli.StringFlag{
			Name:        "order, o",
			Usage:       "order results by ascending/descending",
			Value:       "asc",
			Destination: &order,
		},
		cli.IntFlag{
			Name:        "limit, l",
			Usage:       "how many results to retrieve",
			Value:       100,
			Destination: &limit,
		},
	},
}

func getAuditEvent(c *cli.Context) error {
	if !c.IsSet("start") {
		if c.Args().Get(0) != "" {
			startTime = c.Args().Get(0)
		}
	}

	if !c.IsSet("end") {
		if c.Args().Get(1) != "" {
			endTime = c.Args().Get(1)
		}
	}

	if !c.IsSet("order") {
		if c.Args().Get(2) != "" {
			order = c.Args().Get(2)
		}
	}

	if !c.IsSet("limit") {
		if c.Args().Get(3) != "" {
			limitStr, err := strconv.ParseInt(c.Args().Get(3), 10, 32)
			if err == nil {
				limit = int(limitStr)
			}
		}
	}

	s, err := time.Parse(timeFormat, startTime)
	if err != nil {
		return fmt.Errorf("invalid time format for start: %v", err)
	}

	e, err := time.Parse(timeFormat, endTime)
	if err != nil {
		return fmt.Errorf("invalid time format for end: %v", err)
	}

	if e.Before(s) {
		return errors.New("start cannot be after before")
	}

	conn, err := setupClient()
	if err != nil {
		return err
	}

	defer conn.Close()

	client := gctrpc.NewGoCryptoTraderClient(conn)

	_, offset := time.Now().Zone()
	loc := time.FixedZone("", -offset)

	result, err := client.GetAuditEvent(context.Background(),
		&gctrpc.GetAuditEventRequest{
			StartDate: s.In(loc).Format(timeFormat),
			EndDate:   e.In(loc).Format(timeFormat),
			Limit:     int32(limit),
			OrderBy:   order,
			Offset:    int32(offset),
		})

	if err != nil {
		return err
	}

	jsonOutput(result)
	return nil
}
