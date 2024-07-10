package main

import (
	"fmt"
	"net/http"
)

type Options struct {
	Concurrency bool
	Validate    bool
	MaxDepth    int
	ReturnType  string
	Quality     string
	Binary      bool
	UserAgent   string
}

func FetchFavicons(url string, options Options) ([]string, error) {
	client := &http.Client{}

	commonLocations := []string{
		"/favicon.ico",
	}
	foundFavicons := []string{}
	for i := 0; i < len(commonLocations); i++ {
		urlToGet := url + commonLocations[i]
		req, httpErr := http.NewRequest("GET", urlToGet, nil)
		if httpErr != nil {
			return nil, httpErr
		}
		resp, clientErr := client.Do(req)
		if clientErr != nil {
			return nil, clientErr
		}
		fmt.Println(resp.StatusCode)
		if resp.StatusCode == 200 {
			foundFavicons = append(foundFavicons, urlToGet)
		}
		defer resp.Body.Close()
	}

	fmt.Println("Final slice of favicons:", foundFavicons)
	return foundFavicons, nil
}
