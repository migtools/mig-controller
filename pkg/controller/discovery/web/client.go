package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	liburl "net/url"
)

// Thin client.
// Examples:
//   client := discoveryClient{
//       Token: <token>,
//   }
//   schema := discovery.Schema{}
//   err := client.Get("/schema", &schema)

//
// Thin REST API client.
type Client struct {
	// Bearer token.
	Token string
}

//
// Http GET
func (c *Client) Get(url string, resource interface{}) error {
	header := http.Header{}
	if c.Token != "" {
		header["Authorization"] = []string{
			fmt.Sprintf("Bearer %s", c.Token),
		}
	}
	pURL, err := liburl.Parse(url)
	if err != nil {
		Log.Trace(err)
		return err
	}
	if pURL.Host == "" {
		pURL.Scheme = "http"
		pURL.Host = "localhost:8080"
	}
	request := &http.Request{
		Method: http.MethodGet,
		Header: header,
		URL:    pURL,
	}
	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		Log.Trace(err)
		return err
	}
	if response.StatusCode == http.StatusOK {
		defer response.Body.Close()
		content, err := ioutil.ReadAll(response.Body)
		if err != nil {
			Log.Trace(err)
			return err
		}
		err = json.Unmarshal(content, resource)
		if err != nil {
			Log.Trace(err)
			return err
		}
		return nil
	} else {
		err = errors.New(response.Status)
	}

	return nil
}
