package couchbasecloudclient

import (
	"net/http"
	"net/url"
	"strconv"
)

type ClustersList struct {
	Cursor Cursor  `json:"cursor"`
	Data   []Cluster `json:"data"`
}

type Cluster struct {
	Id        string
	CloudId   string
	Name      string
	Nodes     int
	ProjectId string
	Services  []string
	TenantId  string
}

type ListClustersOptions struct {
	Page      int
	PerPage   int
	SortBy    *string
	CloudId   *string
	ProjectId *string
}

const listClustersUrl = "/clusters"

func (client *CouchbaseCloudClient) ListClusters(options *ListClustersOptions) (*ClustersList, error) {
	cloudsUrl := client.BaseURL + client.getApiEndpoint(listClustersUrl)
	setListClustersParams(&cloudsUrl, *options)

	req, err := http.NewRequest(http.MethodGet, cloudsUrl, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	res := ClustersList{}

	if err := client.sendRequest(req, &res); err != nil {
		return nil, err
	}

	return &res, nil
}

func setListClustersParams(clustersUrl *string, options ListClustersOptions) {
	params := url.Values{}

	if options.Page != 0 {
		params.Add("page", strconv.Itoa(options.Page))
	}

	if options.PerPage != 0 {
		params.Add("perPage", strconv.Itoa(options.PerPage))
	}

	if options.SortBy != nil {
		params.Add("sortBy", *options.SortBy)
	}

	if options.CloudId != nil {
		params.Add("cloudId", *options.CloudId)
	}

	if options.ProjectId != nil {
		params.Add("projectId", *options.ProjectId)
	}

	if urlParams := params.Encode(); urlParams != "" {
		*clustersUrl += "?" + urlParams
	}
}
