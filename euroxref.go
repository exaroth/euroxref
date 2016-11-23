// Package euroxref allows currency conversion between different currencies based on data for last 90 days from European Central Bank.
package euroxref

import (
	"encoding/xml"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// exchangeReferenceRatesUrl defines source url for currency data.
const exchangeReferenceRatesUrl = "https://www.ecb.europa.eu/stats/eurofxref/eurofxref-hist-90d.xml"

// EUCurr is identifier for Euro currency.
const EUCurr = "EUR"

// EuRate is exchange rate for Euro (all other rates are relative to Euro).
const EURate = 1.0

// XRefDateLayout represents date format used by European Central Bank for referencing dates in xml file.
const XRefDateLayout = "2006-01-02"

// RawExchangeRate represents single currency record retrieved from European Cental Bank XML file.
type RawExchangeRate struct {
	Currency string `xml:"currency,attr"`
	Rate     string `xml:"rate,attr"`
}

// XRefRawData represents record for single day containing rate data.
type XRefRawData struct {
	RateTime string            `xml:"time,attr"`
	Rates    []RawExchangeRate `xml:"Cube"`
}

// XRefRawResponse represents exchange rate data retrieved from European Central Bank.
type XRefRawResponse struct {
	XMLName xml.Name
	Data    []XRefRawData `xml:"Cube>Cube"`
}

// Interface representing parsed exchange rate.
type ExchangeRateInterface interface {
	Round(int) float64
}

// exchangeRate represents parsed RawExchangeRate to be passed for processing.
type ExchangeRate struct {
	// Currency representation.
	Currency string
	// Rate for given day.
	Rate float64
}

// Rounds Rate of given Currency based on precision given.
func (e *ExchangeRate) Round(prec int) float64 {
	rounded := FloatToFixed(e.Rate, prec)
	e.Rate = rounded
	return rounded
}

// ExchangeRates represents list of exchangeRate structs.
type ExchangeRates []ExchangeRate

// Map converts collection of exchangeRate structs into more readable format.
func (e ExchangeRates) Map() map[string]float64 {
	res := make(map[string]float64)
	for _, v := range e {
		res[v.Currency] = v.Rate
	}
	return res
}

// newExchangeRate returns new populated exchangeRate instance.
func newExchangeRate(r *RawExchangeRate) (rate ExchangeRateInterface, err error) {
	var v float64
	v, err = strconv.ParseFloat(r.Rate, 32)
	if err != nil {
		return rate, errors.New(fmt.Sprintf("Invalid input rate value for %s, %s", r.Currency, r.Rate))
	}
	return &ExchangeRate{
		Currency: r.Currency,
		Rate:     v,
	}, nil
}

// XRefInterface represents basic interface used for fetching and converting exchange rates.
type XRefInterface interface {
	fetchXML() error
	round(float64, ...int) float64
	computeExchangeValue(float64, *ExchangeRate, *ExchangeRate) (float64, error)
	Convert(float64, string, string, time.Time) (float64, error)
	Fetch(time.Time) (ExchangeRates, error)
	FetchAll() (map[time.Time]ExchangeRates, error)
}

// Client containing all data required for interaction with euroxref.
type Client struct {
	// HTTP client used for retrieving data.
	HTTPClient *http.Client
	// Fetched currency exchange data.
	XRefData *XRefRawResponse
	// Amount of time in seconds after which exchange list will be refreshed. If set to 0 list of currencies are refreshed every time.
	RefreshInterval int
	// Precision to be used for computational rounding of values.
	prec int
	// Last time when data was fetched from remote server.
	lastFetched time.Time
}

// New() returns new instance of XRefInterface.
// precision paramenter defines float precision when calculating exchange rates.
// refresh interval defines how often (in seconds) xml data will be downloaded after last fetch
// from the server, if set to 0, data will be fetched every time.
func New(precision, refreshInterval uint) (client XRefInterface) {
	return &Client{
		HTTPClient:      http.DefaultClient,
		prec:            int(precision),
		RefreshInterval: int(refreshInterval),
	}
}

// roundFloat rounds the float into nearest integer.
func roundFloat(num float64) uint64 {
	const roundBarrier = 0.5
	return uint64(num + math.Copysign(roundBarrier, num))
}

// FloatToFixed rounds floating number based on precision of computation.
func FloatToFixed(num float64, prec int) float64 {
	// Force precision to be at least one
	if prec < 1 {
		prec = 1
	}
	exp := math.Pow(10, float64(prec))
	return float64(roundFloat(num*exp)) / exp
}

// FetchXML retrieves xml containing currency Data and parses it into XRefRawResponse
func (c *Client) fetchXML() (err error) {
	// If Refresh interval is greater than 0 and it's greater than time elapsed from last fetch
	// don't download data again.
	if (int(time.Now().Sub(c.lastFetched).Seconds()) < c.RefreshInterval) && (c.RefreshInterval > 0) {
		return
	}
	resp, err := http.Get(exchangeReferenceRatesUrl)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	data := &XRefRawResponse{}
	err = xml.NewDecoder(resp.Body).Decode(data)
	c.XRefData = data
	c.lastFetched = time.Now()
	return
}

// round rounds the value based on precision set during client initialization
// or precision passed as optional arg.
func (c *Client) round(num float64, params ...int) float64 {
	if len(params) > 0 {
		prec := params[0]
		return FloatToFixed(num, prec)
	} else {
		return FloatToFixed(num, c.prec)

	}
}

// computeExchangeValue returns computed value of exchange rate between 2 currencies
// and passed value.
func (c *Client) computeExchangeValue(amount float64, in, to *ExchangeRate) (result float64, err error) {
	if amount < 0 {
		return result, errors.New("Amount of conversion currency can't be negative")
	}
	// If currencies are the same there's no need to perform any computation.
	if in.Currency == to.Currency {
		result = c.round(amount)
		return
	}
	// Computation of exchange rate between currency A and B is performed by eliminating common denominator of EUR value as all exchange rates are relative to it. ((rateB/rateEUR)/(rateA/rateEUR)) == ((rateB/rateEUR) * (rateEUR/rateA)) == (rateB/rateA)
	return c.round(c.round(amount, 2) * c.round(to.Rate/in.Rate)), nil
}

// Convert is main method for computing exchange rates between currencies.
// amount is nominal amount of first currency.
// source and target define currencies to compute exchange rates for.
// t defines time for which exchange rates will be fetched.
func (c *Client) Convert(amount float64, source, target string, t time.Time) (result float64, err error) {
	var dayData ExchangeRates
	var in, to *ExchangeRate
	dayData, err = c.Fetch(t)
	if err != nil {
		return
	}
	for idx, rec := range dayData {
		if source == rec.Currency {
			in = &dayData[idx]
		}
		if target == rec.Currency {
			to = &dayData[idx]
		}
	}
	// As EUR is a reference point to all other rates
	// It doesn't show up on currency lists but we still want
	// to support it.
	if source == EUCurr || target == EUCurr {
		euRec := &ExchangeRate{
			Currency: EUCurr,
			Rate:     EURate,
		}
		if source == EUCurr {
			in = euRec
		}
		if target == EUCurr {
			to = euRec
		}
	}
	if in == nil || to == nil {
		var availableCurrencies []string
		for _, rec := range dayData {
			availableCurrencies = append(availableCurrencies, rec.Currency)
		}
		return result, errors.New(fmt.Sprintf("Invalid currencies selected: %s, %s. List of available currency rates: %s for %s", source, target, strings.Join(availableCurrencies, ", "), t.Format(XRefDateLayout)))
	}
	return c.computeExchangeValue(amount, in, to)
}

// Fetch retrieves collection of exchangeRate values for given month.
func (c *Client) Fetch(t time.Time) (rates ExchangeRates, err error) {
	timeKey := t.Format(XRefDateLayout)
	var dayData []RawExchangeRate
	err = c.fetchXML()
	if err != nil {
		return
	}
	for _, dayD := range c.XRefData.Data {
		if dayD.RateTime == timeKey {
			dayData = dayD.Rates
			break
		}
	}
	if len(dayData) == 0 {
		return rates, errors.New(fmt.Sprintf("Currency data for %s doesn't exist. Records are only available for past 90 days, excluding present day.", timeKey))
	}
	rates = ExchangeRates{}
	var temp interface{}
	for _, rec := range dayData {
		temp, err = newExchangeRate(&rec)
		if err != nil {
			return rates, err
		}
		// We can skip checking if value was casted succesfully here
		val, _ := temp.(*ExchangeRate)
		val.Round(c.prec)
		rates = append(rates, *val)
	}
	return
}

// FetchAll retrieves all available exchangeRate records.
func (c *Client) FetchAll() (rates map[time.Time]ExchangeRates, err error) {
	err = c.fetchXML()
	if err != nil {
		return
	}
	rates = make(map[time.Time]ExchangeRates)
	var t time.Time
	var d ExchangeRates
	for _, dayD := range c.XRefData.Data {
		t, err = time.Parse(XRefDateLayout, dayD.RateTime)
		if err != nil {
			return
		}
		d, err = c.Fetch(t)
		if err != nil {
			return
		}
		rates[t] = d
	}
	return

}
