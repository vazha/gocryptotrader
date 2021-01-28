// Open Exchange Rates provides a simple, lightweight and portable JSON API with
// live and historical foreign exchange (forex) rates, via a simple and
// easy-to-integrate API, in JSON format. Data are tracked and blended
// algorithmically from multiple reliable sources, ensuring fair and unbiased
// consistency.
// End-of-day rates are available historically for all days going back to
// 1st January, 1999.

package openexchangerates

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/vazha/gocryptotrader/common"
	"github.com/vazha/gocryptotrader/currency/forexprovider/base"
	"github.com/vazha/gocryptotrader/exchanges/request"
	"github.com/vazha/gocryptotrader/log"
)

// Setup sets values for the OXR object
func (o *OXR) Setup(config base.Settings) error {
	if config.APIKeyLvl < 0 || config.APIKeyLvl > 2 {
		log.Errorf(log.Global,
			"apikey incorrectly set in config.json for %s, please set appropriate account levels\n",
			config.Name)
		return errors.New("apikey set failure")
	}
	o.APIKey = config.APIKey
	o.APIKeyLvl = config.APIKeyLvl
	o.Enabled = config.Enabled
	o.Name = config.Name
	o.RESTPollingDelay = config.RESTPollingDelay
	o.Verbose = config.Verbose
	o.PrimaryProvider = config.PrimaryProvider
	o.Requester = request.New(o.Name,
		common.NewHTTPClientWithTimeout(base.DefaultTimeOut))
	return nil
}

// GetRates is a wrapper function to return rates
func (o *OXR) GetRates(baseCurrency, symbols string) (map[string]float64, error) {
	rates, err := o.GetLatest(baseCurrency, symbols, false, false)
	if err != nil {
		return nil, err
	}

	standardisedRates := make(map[string]float64)
	for k, v := range rates {
		curr := baseCurrency + k
		standardisedRates[curr] = v
	}

	return standardisedRates, nil
}

// GetLatest returns the latest exchange rates available from the Open Exchange
// Rates
func (o *OXR) GetLatest(baseCurrency, symbols string, prettyPrint, showAlternative bool) (map[string]float64, error) {
	var resp Latest

	v := url.Values{}
	v.Set("base", baseCurrency)
	v.Set("symbols", symbols)
	v.Set("prettyprint", strconv.FormatBool(prettyPrint))
	v.Set("show_alternative", strconv.FormatBool(showAlternative))

	if err := o.SendHTTPRequest(APIEndpointLatest, v, &resp); err != nil {
		return nil, err
	}

	if resp.Error {
		return nil, errors.New(resp.Message)
	}
	return resp.Rates, nil
}

// GetHistoricalRates returns historical exchange rates for any date available
// from the Open Exchange Rates API.
func (o *OXR) GetHistoricalRates(date, baseCurrency string, symbols []string, prettyPrint, showAlternative bool) (map[string]float64, error) {
	var resp Latest

	v := url.Values{}
	v.Set("base", baseCurrency)
	v.Set("symbols", strings.Join(symbols, ","))
	v.Set("prettyprint", strconv.FormatBool(prettyPrint))
	v.Set("show_alternative", strconv.FormatBool(showAlternative))
	endpoint := fmt.Sprintf(APIEndpointHistorical, date)

	if err := o.SendHTTPRequest(endpoint, v, &resp); err != nil {
		return nil, err
	}

	if resp.Error {
		return nil, errors.New(resp.Message)
	}
	return resp.Rates, nil
}

// GetCurrencies returns a list of all currency symbols available from the Open
// Exchange Rates API,
func (o *OXR) GetCurrencies(showInactive, prettyPrint, showAlternative bool) (map[string]string, error) {
	resp := make(map[string]string)

	v := url.Values{}
	v.Set("show_inactive", strconv.FormatBool(showInactive))
	v.Set("prettyprint", strconv.FormatBool(prettyPrint))
	v.Set("show_alternative", strconv.FormatBool(showAlternative))

	return resp, o.SendHTTPRequest(APIEndpointCurrencies, v, &resp)
}

// GetSupportedCurrencies returns a list of supported currencies
func (o *OXR) GetSupportedCurrencies() ([]string, error) {
	return strings.Split(oxrSupportedCurrencies, ","), nil
}

// GetTimeSeries returns historical exchange rates for a given time period,
// where available.
func (o *OXR) GetTimeSeries(baseCurrency, startDate, endDate string, symbols []string, prettyPrint, showAlternative bool) (map[string]interface{}, error) {
	if o.APIKeyLvl < APIEnterpriseAccess {
		return nil, errors.New("upgrade account, insufficient access")
	}

	var resp TimeSeries

	v := url.Values{}
	v.Set("base", baseCurrency)
	v.Set("start", startDate)
	v.Set("end", endDate)
	v.Set("symbols", strings.Join(symbols, ","))
	v.Set("prettyprint", strconv.FormatBool(prettyPrint))
	v.Set("show_alternative", strconv.FormatBool(showAlternative))

	if err := o.SendHTTPRequest(APIEndpointTimeSeries, v, &resp); err != nil {
		return nil, err
	}

	if resp.Error {
		return nil, errors.New(resp.Message)
	}
	return resp.Rates, nil
}

// ConvertCurrency converts any money value from one currency to another at the
// latest API rates
func (o *OXR) ConvertCurrency(amount float64, from, to string) (float64, error) {
	if o.APIKeyLvl < APIUnlimitedAccess {
		return 0, errors.New("upgrade account, insufficient access")
	}

	var resp Convert

	endPoint := fmt.Sprintf(APIEndpointConvert, strconv.FormatFloat(amount, 'f', -1, 64), from, to)
	if err := o.SendHTTPRequest(endPoint, url.Values{}, &resp); err != nil {
		return 0, err
	}

	if resp.Error {
		return 0, errors.New(resp.Message)
	}
	return resp.Response, nil
}

// GetOHLC returns historical Open, High Low, Close (OHLC) and Average exchange
// rates for a given time period, ranging from 1 month to 1 minute, where
// available.
func (o *OXR) GetOHLC(startTime, period, baseCurrency string, symbols []string, prettyPrint bool) (map[string]interface{}, error) {
	if o.APIKeyLvl < APIUnlimitedAccess {
		return nil, errors.New("upgrade account, insufficient access")
	}

	var resp OHLC

	v := url.Values{}
	v.Set("start_time", startTime)
	v.Set("period", period)
	v.Set("base", baseCurrency)
	v.Set("symbols", strings.Join(symbols, ","))
	v.Set("prettyprint", strconv.FormatBool(prettyPrint))

	if err := o.SendHTTPRequest(APIEndpointOHLC, v, &resp); err != nil {
		return nil, err
	}

	if resp.Error {
		return nil, errors.New(resp.Message)
	}
	return resp.Rates, nil
}

// GetUsageStats returns basic plan information and usage statistics for an Open
// Exchange Rates App ID
func (o *OXR) GetUsageStats(prettyPrint bool) (Usage, error) {
	var resp Usage

	v := url.Values{}
	v.Set("prettyprint", strconv.FormatBool(prettyPrint))

	if err := o.SendHTTPRequest(APIEndpointUsage, v, &resp); err != nil {
		return resp, err
	}

	if resp.Error {
		return resp, errors.New(resp.Message)
	}
	return resp, nil
}

// SendHTTPRequest sends a HTTP request
func (o *OXR) SendHTTPRequest(endpoint string, values url.Values, result interface{}) error {
	headers := make(map[string]string)
	headers["Authorization"] = "Token " + o.APIKey
	path := APIURL + endpoint + "?" + values.Encode()

	return o.Requester.SendPayload(context.Background(), &request.Item{
		Method:  http.MethodGet,
		Path:    path,
		Result:  result,
		Verbose: o.Verbose})
}
