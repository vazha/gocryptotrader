package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/vazha/gocryptotrader/common"
	"github.com/vazha/gocryptotrader/config"
	"github.com/vazha/gocryptotrader/currency"
	"github.com/vazha/gocryptotrader/exchanges/asset"
)

const (
	packageTests   = "%s_test.go"
	packageTypes   = "%s_types.go"
	packageWrapper = "%s_wrapper.go"
	packageMain    = "%s.go"
	packageReadme  = "README.md"

	exchangePackageLocation = "../../exchanges"
	exchangeConfigPath      = "../../testdata/configtest.json"
)

var (
	exchangeDirectory string
	exchangeTest      string
	exchangeTypes     string
	exchangeWrapper   string
	exchangeMain      string
	exchangeReadme    string
)

type exchange struct {
	Name        string
	CapitalName string
	Variable    string
	REST        bool
	WS          bool
	FIX         bool
}

func main() {
	var newExchangeName string
	var websocketSupport, restSupport, fixSupport bool

	flag.StringVar(&newExchangeName, "name", "", "-name [string] adds a new exchange")
	flag.BoolVar(&websocketSupport, "ws", false, "-websocket adds websocket support")
	flag.BoolVar(&restSupport, "rest", false, "-rest adds REST support")
	flag.BoolVar(&fixSupport, "fix", false, "-fix adds FIX support?")

	flag.Parse()

	fmt.Println("GoCryptoTrader: Exchange templating tool.")

	if newExchangeName == "" || newExchangeName == " " {
		log.Fatal(`GoCryptoTrader: Exchange templating tool exchange name not set e.g. "exchange_template -name [newExchangeNameString]"`)
	}

	if !websocketSupport && !restSupport && !fixSupport {
		log.Fatal(`GoCryptoTrader: Exchange templating tool support not set e.g. "exchange_template -name [newExchangeNameString] [-fix -ws -rest]"`)
	}

	fmt.Println("Exchange Name: ", newExchangeName)
	fmt.Println("Websocket Supported: ", websocketSupport)
	fmt.Println("REST Supported: ", restSupport)
	fmt.Println("FIX Supported: ", fixSupport)
	fmt.Println()
	fmt.Println("Please check if everything is correct and then type y to continue or n to cancel...")

	var choice []byte
	_, err := fmt.Scanln(&choice)
	if err != nil {
		log.Fatal("GoCryptoTrader: Exchange templating tool fmt.Scanln ", err)
	}

	if !common.YesOrNo(string(choice)) {
		log.Fatal("GoCryptoTrader: Exchange templating tool stopped...")
	}

	newExchangeName = strings.ToLower(newExchangeName)
	v := newExchangeName[:1]
	capName := strings.ToUpper(v) + newExchangeName[1:]

	exch := exchange{
		Name:        newExchangeName,
		CapitalName: capName,
		Variable:    v,
		REST:        restSupport,
		WS:          websocketSupport,
		FIX:         fixSupport,
	}

	configTestFile := config.GetConfig()
	err = configTestFile.LoadConfig(exchangeConfigPath, true)
	if err != nil {
		log.Fatal("GoCryptoTrader: Exchange templating configuration retrieval error ", err)
	}
	// NOTE need to nullify encrypt configuration

	var configTestExchanges []string
	for x := range configTestFile.Exchanges {
		configTestExchanges = append(configTestExchanges, configTestFile.Exchanges[x].Name)
	}

	if common.StringDataContainsInsensitive(configTestExchanges, capName) {
		log.Fatal("GoCryptoTrader: Exchange templating configuration error - exchange already exists")
	}

	newExchConfig := config.ExchangeConfig{}
	newExchConfig.Name = capName
	newExchConfig.Enabled = true
	newExchConfig.API.Credentials.Key = "Key"
	newExchConfig.API.Credentials.Secret = "Secret"
	newExchConfig.CurrencyPairs = &currency.PairsManager{
		AssetTypes: asset.Items{
			asset.Spot,
		},
		UseGlobalFormat: true,
		RequestFormat: &currency.PairFormat{
			Uppercase: true,
		},
		ConfigFormat: &currency.PairFormat{
			Uppercase: true,
		},
	}

	configTestFile.Exchanges = append(configTestFile.Exchanges, newExchConfig)

	err = configTestFile.SaveConfig(exchangeConfigPath, false)
	if err != nil {
		log.Fatal("GoCryptoTrader: Exchange templating configuration error - cannot save")
	}

	exchangeDirectory = filepath.Join(exchangePackageLocation, newExchangeName)
	exchangeTest = filepath.Join(exchangeDirectory, fmt.Sprintf(packageTests, newExchangeName))
	exchangeTypes = filepath.Join(exchangeDirectory, fmt.Sprintf(packageTypes, newExchangeName))
	exchangeWrapper = filepath.Join(exchangeDirectory, fmt.Sprintf(packageWrapper, newExchangeName))
	exchangeMain = filepath.Join(exchangeDirectory, fmt.Sprintf(packageMain, newExchangeName))
	exchangeReadme = filepath.Join(exchangeDirectory, packageReadme)

	err = os.Mkdir(exchangeDirectory, 0700)
	if err != nil {
		log.Fatal("GoCryptoTrader: Exchange templating tool cannot make directory ", err)
	}

	tReadme, err := template.New("readme").ParseFiles("readme_file.tmpl")
	if err != nil {
		log.Fatal("GoCryptoTrader: Exchange templating tool error ", err)
	}
	newFile(exchangeReadme)
	r1, err := os.OpenFile(exchangeReadme, os.O_WRONLY, 0700)
	if err != nil {
		log.Fatal("GoCryptoTrader: Exchange templating tool cannot open file ", err)
	}
	tReadme.Execute(r1, exch)
	r1.Close()

	tMain, err := template.New("main").ParseFiles("main_file.tmpl")
	if err != nil {
		log.Fatal("GoCryptoTrader: Exchange templating tool error ", err)
	}
	newFile(exchangeMain)
	m1, err := os.OpenFile(exchangeMain, os.O_WRONLY, 0700)
	if err != nil {
		log.Fatal("GoCryptoTrader: Exchange templating tool cannot open file ", err)
	}
	tMain.Execute(m1, exch)
	m1.Close()

	tTest, err := template.New("test").ParseFiles("test_file.tmpl")
	if err != nil {
		log.Fatal("GoCryptoTrader: Exchange templating tool error ", err)
	}
	newFile(exchangeTest)
	t1, err := os.OpenFile(exchangeTest, os.O_WRONLY, 0700)
	if err != nil {
		log.Fatal("GoCryptoTrader: Exchange templating tool cannot open file ", err)
	}
	tTest.Execute(t1, exch)
	t1.Close()

	tType, err := template.New("type").ParseFiles("type_file.tmpl")
	if err != nil {
		log.Fatal("GoCryptoTrader: Exchange templating tool error ", err)
	}
	newFile(exchangeTypes)
	ty1, err := os.OpenFile(exchangeTypes, os.O_WRONLY, 0700)
	if err != nil {
		log.Fatal("GoCryptoTrader: Exchange templating tool cannot open file ", err)
	}
	tType.Execute(ty1, exch)
	ty1.Close()

	tWrapper, err := template.New("wrapper").ParseFiles("wrapper_file.tmpl")
	if err != nil {
		log.Fatal("GoCryptoTrader: Exchange templating tool error ", err)
	}
	newFile(exchangeWrapper)
	w1, err := os.OpenFile(exchangeWrapper, os.O_WRONLY, 0700)
	if err != nil {
		log.Fatal("GoCryptoTrader: Exchange templating tool cannot open file ", err)
	}
	tWrapper.Execute(w1, exch)
	w1.Close()

	err = exec.Command("go", "fmt", exchangeDirectory).Run()
	if err != nil {
		log.Fatal("GoCryptoTrader: Exchange templating tool go fmt error ", err)
	}

	err = exec.Command("go", "test", exchangeDirectory).Run()
	if err != nil {
		log.Fatal("GoCryptoTrader: Exchange templating tool testing failed ", err)
	}

	fmt.Println("GoCryptoTrader: Exchange templating tool service complete")
	fmt.Println("When the exchange code implementation has been completed (REST/Websocket/wrappers and tests), please add the exchange to engine/exchange.go")
	fmt.Println("Add the exchange config settings to config_example.json (it will automatically be added to testdata/configtest.json)")
	fmt.Println("Increment the available exchanges counter in config/config_test.go")
	fmt.Println("Add the exchange name to exchanges/support.go")
	fmt.Println("Ensure go test ./... -race passes")
	fmt.Println("Open a pull request")
	fmt.Println("If help is needed, please post a message in Slack.")
}

func newFile(path string) {
	_, err := os.Stat(path)

	if os.IsNotExist(err) {
		var file, err = os.Create(path)
		if err != nil {
			log.Fatal("GoCryptoTrader: Exchange templating tool file creation error ", err)
		}
		file.Close()
	}
}
