package euroxref_test

import (
	"encoding/xml"
	"github.com/exaroth/euroxref-konrad"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"
)

var testResponse = &euroxref.XRefRawResponse{
	Data: []euroxref.XRefRawData{
		{
			RateTime: "2016-11-11",
			Rates: []euroxref.RawExchangeRate{
				{
					Currency: "USD",
					Rate:     "1.002",
				},
				{
					Currency: "CHF",
					Rate:     "1.03",
				},
				{
					Currency: "PLN",
					Rate:     "0.321",
				},
				{
					Currency: "XYZ",
					Rate:     "1.9999999",
				},
			},
		},
		{
			RateTime: "2016-11-10",
			Rates: []euroxref.RawExchangeRate{
				{
					Currency: "USD",
					Rate:     "1.003123142",
				},
				{
					Currency: "PLN",
					Rate:     "0.3211231231",
				},
				{
					Currency: "XYZ",
					Rate:     "2.00001999",
				},
			},
		},
		{
			RateTime: "2016-11-09",
			Rates: []euroxref.RawExchangeRate{
				{
					Currency: "USD",
					Rate:     "2.999999", // Trump elected :)
				},
			},
		},
		{
			RateTime: "2016-11-08",
			Rates:    []euroxref.RawExchangeRate{},
		},
	},
}

type MockedTransport struct {
	Transport http.Transport
}

func (mt *MockedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http" // Disable ssl
	return mt.Transport.RoundTrip(req)
}

func MockServer(t *testing.T, c *euroxref.Client, h http.HandlerFunc) *httptest.Server {
	mockedServer := httptest.NewServer(http.HandlerFunc(h))
	c.HTTPClient.Transport = &MockedTransport{
		Transport: http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return url.Parse(mockedServer.URL)
			},
		},
	}
	return mockedServer
}

func testHandle(reqURL, reqMethod, reqBody *string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		body, _ := ioutil.ReadAll(req.Body)
		*reqBody = string(body)
		*reqURL = req.URL.String()
		*reqMethod = req.Method
		data, err := xml.Marshal(testResponse)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := w.Write(data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func TestFetch(t *testing.T) {
	tests := []struct {
		Date      time.Time
		Expected  *euroxref.ExchangeRates
		Precision uint
		Err       bool
	}{
		{
			Date: time.Date(2016, time.November, 11, 23, 0, 0, 0, time.UTC),
			Expected: &euroxref.ExchangeRates{
				{
					Currency: "USD",
					Rate:     1.002,
				},
				{
					Currency: "CHF",
					Rate:     1.03,
				},
				{
					Currency: "PLN",
					Rate:     0.321,
				},
				{
					Currency: "XYZ",
					Rate:     2,
				},
			},
			Precision: 4,
			Err:       false,
		},
		{
			Date: time.Date(2016, time.November, 10, 23, 0, 0, 0, time.UTC),
			Expected: &euroxref.ExchangeRates{
				{
					Currency: "USD",
					Rate:     1.0031,
				},
				{
					Currency: "PLN",
					Rate:     0.3211,
				},
				{
					Currency: "XYZ",
					Rate:     2,
				},
			},
			Precision: 4,
			Err:       false,
		},
		{
			Date: time.Date(2016, time.November, 9, 23, 0, 0, 0, time.UTC),
			Expected: &euroxref.ExchangeRates{
				{
					Currency: "USD",
					Rate:     3,
				},
			},
			Precision: 4,
			Err:       false,
		},
		{
			Date:      time.Date(2016, time.November, 8, 23, 0, 0, 0, time.UTC),
			Expected:  nil,
			Precision: 4,
			Err:       true,
		},
		{
			Date:      time.Date(2017, time.November, 8, 23, 0, 0, 0, time.UTC),
			Precision: 4,
			Expected:  nil,
			Err:       true,
		},
		{
			Date:      time.Date(2016, time.February, 7, 23, 0, 0, 0, time.UTC),
			Precision: 4,
			Expected:  nil,
			Err:       true,
		},
		{
			Date: time.Date(2016, time.November, 10, 23, 0, 0, 0, time.UTC),
			Expected: &euroxref.ExchangeRates{
				{
					Currency: "USD",
					Rate:     1.003123,
				},
				{
					Currency: "PLN",
					Rate:     0.321123,
				},
				{
					Currency: "XYZ",
					Rate:     2.00002,
				},
			},
			Precision: 6,
			Err:       false,
		},
	}
	for i, test := range tests {
		var reqUrl, reqMethod, reqBody string
		handler := testHandle(&reqUrl, &reqMethod, &reqBody)
		client := euroxref.New(test.Precision, 0)
		mock := MockServer(t, client.(*euroxref.Client), handler)
		defer mock.Close()
		res, err := client.Fetch(test.Date)
		if test.Err {
			if err == nil {
				t.Errorf("Want err != nil; got nil (i:%d)", i)
			}
		} else {
			if err != nil {
				t.Errorf("Want err == nil; got %v (i:%d)", err, i)
			}
			if !reflect.DeepEqual(*test.Expected, res) {
				t.Errorf("Values `%v` and `%v` are not equal (i:%d)", *test.Expected, res, i)
			}
		}
	}
}

func TestConvert(t *testing.T) {

	tests := []struct {
		Date       time.Time
		Amount     float64
		Precision  uint
		Currencies [2]string
		Expected   float64
		Err        bool
	}{
		{
			Date:       time.Date(2016, time.November, 11, 23, 0, 0, 0, time.UTC),
			Amount:     10,
			Precision:  4,
			Currencies: [2]string{"CHF", "USD"},
			Expected:   9.728,
			Err:        false,
		},
		{
			Date:       time.Date(2016, time.November, 10, 23, 0, 0, 0, time.UTC),
			Amount:     10,
			Precision:  4,
			Currencies: [2]string{"USD", "XYZ"},
			Expected:   19.938,
			Err:        false,
		},
		// {
		// 	Date:       time.Date(2016, time.November, 10, 23, 0, 0, 0, time.UTC),
		// 	Amount:     10021000000.8999999,
		// 	Precision:  6,
		// 	Currencies: [2]string{"USD", "XYZ"},
		// 	Expected:   19.93793,
		// 	Err:        false,
		// },
		{
			Date:       time.Date(2016, time.November, 10, 23, 0, 0, 0, time.UTC),
			Amount:     10,
			Precision:  6,
			Currencies: [2]string{"EUR", "USD"},
			Expected:   10.03123,
			Err:        false,
		},
		{
			Date:       time.Date(2016, time.November, 10, 23, 0, 0, 0, time.UTC),
			Amount:     10.50,
			Precision:  6,
			Currencies: [2]string{"EUR", "EUR"},
			Expected:   10.50,
			Err:        false,
		},
		{
			Date:       time.Date(2016, time.November, 11, 23, 0, 0, 0, time.UTC),
			Amount:     10,
			Precision:  4,
			Currencies: [2]string{"USD", "USD"},
			Expected:   10,
			Err:        false,
		},
		{
			Date:       time.Date(2016, time.November, 11, 23, 0, 0, 0, time.UTC),
			Amount:     10,
			Precision:  0,
			Currencies: [2]string{"PLN", "CHF"},
			Expected:   33,
			Err:        false,
		},
		{
			Date:       time.Date(2016, time.November, 11, 23, 0, 0, 0, time.UTC),
			Amount:     10,
			Precision:  4,
			Currencies: [2]string{"BLE", "USD"},
			Expected:   10,
			Err:        true,
		},
		{
			Date:       time.Date(2016, time.November, 11, 23, 0, 0, 0, time.UTC),
			Amount:     10,
			Precision:  4,
			Currencies: [2]string{"USD", "BLE"},
			Expected:   0,
			Err:        true,
		},
		{
			Date:       time.Date(2016, time.November, 9, 23, 0, 0, 0, time.UTC),
			Amount:     10,
			Precision:  4,
			Currencies: [2]string{"USD", "PLN"},
			Expected:   0,
			Err:        true,
		},
		{
			Date:       time.Date(2016, time.November, 8, 23, 0, 0, 0, time.UTC),
			Amount:     10,
			Precision:  4,
			Currencies: [2]string{"USD", "PLN"},
			Expected:   0,
			Err:        true,
		},
		{
			Date:       time.Date(2002, time.November, 8, 23, 0, 0, 0, time.UTC),
			Amount:     10,
			Precision:  4,
			Currencies: [2]string{"USD", "PLN"},
			Expected:   0,
			Err:        true,
		},
		{
			Date:       time.Date(2016, time.November, 11, 23, 0, 0, 0, time.UTC),
			Amount:     -10,
			Precision:  4,
			Currencies: [2]string{"USD", "USD"},
			Expected:   0,
			Err:        true,
		},
	}
	for i, test := range tests {
		var reqUrl, reqMethod, reqBody string
		handler := testHandle(&reqUrl, &reqMethod, &reqBody)
		client := euroxref.New(test.Precision, 0)
		mock := MockServer(t, client.(*euroxref.Client), handler)
		defer mock.Close()
		res, err := client.Convert(test.Amount, test.Currencies[0], test.Currencies[1], test.Date)
		if test.Err {
			if err == nil {
				t.Errorf("Want err != nil; got nil (i:%d)", i)
			}
		} else {
			if err != nil {
				t.Errorf("Want err == nil; got %v (i:%d)", err, i)
			}
			if test.Expected != res {
				t.Errorf("Values `%v` and `%v` are not equal (i:%d)", test.Expected, res, i)
			}
		}
	}
}

func TestRoundingFloats(t *testing.T) {
	tests := []struct {
		Value     float64
		Expected  float64
		Precision int
	}{
		{
			Value:     10,
			Expected:  10,
			Precision: 1,
		},
		{
			Value:     10,
			Expected:  10,
			Precision: 4,
		},
		{
			Value:     10,
			Expected:  10,
			Precision: 4,
		},
		{
			Value:     10,
			Expected:  10,
			Precision: 0,
		},
		{
			Value:     0.4233,
			Expected:  0.42,
			Precision: 2,
		},
		{
			Value:     0.4251,
			Expected:  0.43,
			Precision: 2,
		},
		{
			Value:     0.4251,
			Expected:  0.4,
			Precision: 1,
		},
		{
			Value:     0.4851,
			Expected:  0.5,
			Precision: 0, // precision is always set to at least one
		},
		{
			Value:     0.000001,
			Expected:  0,
			Precision: 0,
		},
		{
			Value:     0.425176543,
			Expected:  0.425176543,
			Precision: 10,
		},
		{
			Value:     1.999999999,
			Expected:  1.999999999,
			Precision: 10,
		},
		{
			Value:     0.0000000001,
			Expected:  0.0000000001,
			Precision: 10,
		},
		{
			Value:     2131123123131.222,
			Expected:  2131123123131.222,
			Precision: 3,
		},
	}
	for i, test := range tests {
		result := euroxref.FloatToFixed(test.Value, test.Precision)
		if test.Expected != result {
			t.Errorf("Values `%.10f` and `%.10f` are not equal (i:%d)", test.Expected, result, i)
		}
	}

}
