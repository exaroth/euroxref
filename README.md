# EuroXref

## Description

Small library used for fetching and convering data from European Bank, based on data from the last 90 days

## Example Usage
``` bash
go get github.com/exaroth/euroxref;
```

``` go
    import (
        "fmt"
        "github.com/smallpdf/euroxref-konrad"
        "log"
        "time"
    )
	client := euroxref.New(4, 60) // Set precision to 4 and set interval for fetching xml data to 60 seconds
	// Convert 2.23 Swiss franks to USD on 10th of November 2016.
	result, err := client.Convert(2.23, "CHF", "USD", time.Date(2016, time.November, 10, 23, 0, 0, 0, time.UTC))
	if err != nil {
		log.Printf(err.Error())
	}
	fmt.Println(fmt.Sprintf("%.4f", result))
	// Convert 1000 Euro to PLN for exchange rates from 20th of October.
	result, err = client.Convert(1000, "EUR", "PLN", time.Date(2016, time.October, 20, 23, 0, 0, 0, time.UTC))
	if err != nil {
		log.Printf(err.Error())
	}
	fmt.Println(fmt.Sprintf("%.4f", result))
	// Fetch currency exchange data for 10th of November 2016
	dayData, err := client.Fetch(time.Date(2016, time.November, 10, 23, 0, 0, 0, time.UTC))
	if err != nil {
		log.Printf(err.Error())
	}
	fmt.Println(dayData.Map())
	// Fetch data for last 90 days
	allData, err := client.FetchAll()
	if err != nil {
		log.Printf(err.Error())
	}
	for _, data := range allData {
		fmt.Println(data.Map())
	}
```
