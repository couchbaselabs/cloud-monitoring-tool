package couchbasecloudclient

import (
	"net/http"
	"net/url"
	"strconv"
)

type CloudsList struct {
	Cursor Cursor  `json:"cursor"`
	Data   []Cloud `json:"data"`
}

type Cloud struct {
	Id                 string
	Name               string
	Provider           string
	Region             string
	Status             string
	VirtualNetworkCIDR string
	VirtualNetworkID   string
}

type ListCloudsOptions struct {
	Page    int
	PerPage int
	SortBy  *string
}

const listCloudsUrl = "/clouds"

func (client *CouchbaseCloudClient) ListClouds(options *ListCloudsOptions) (*CloudsList, error) {
	cloudsUrl := client.BaseURL + client.getApiEndpoint(listCloudsUrl)
	setListCloudsParams(&cloudsUrl, *options)

	req, err := http.NewRequest(http.MethodGet, cloudsUrl, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	res := CloudsList{}

	if err := client.sendRequest(req, &res); err != nil {
		return nil, err
	}

	return &res, nil
}

func setListCloudsParams(urlStr *string, options ListCloudsOptions) {
	params := url.Values{}

	if options.SortBy != nil {
		params.Add("sortBy", *options.SortBy)
	}

	if options.Page != 0 {
		params.Add("page", strconv.Itoa(options.Page))
	}

	if options.PerPage != 0 {
		params.Add("perPage", strconv.Itoa(options.PerPage))
	}

	if urlParams := params.Encode(); urlParams != "" {
		*urlStr += "?" + urlParams
	}
}
