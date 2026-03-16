package repository

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestClient(fn roundTripFunc) *http.Client {
	return &http.Client{Transport: fn}
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestElasticsearchRepository_EnsureIndex_CreatesMissingIndex(t *testing.T) {
	var created bool
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		switch req.Method + " " + req.URL.Path {
		case http.MethodHead + " /products":
			return jsonResponse(http.StatusNotFound, ""), nil
		case http.MethodPut + " /products":
			created = true
			return jsonResponse(http.StatusOK, `{}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	repo := NewElasticsearchRepository("http://example.test", "products", client)
	err := repo.EnsureIndex(context.Background())
	require.NoError(t, err)
	assert.True(t, created)
}

func TestElasticsearchRepository_Search(t *testing.T) {
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, req.Method)
		require.Equal(t, "/products/_search", req.URL.Path)

		var body map[string]any
		require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
		assert.Equal(t, float64(20), body["size"])

		return jsonResponse(http.StatusOK, `{
			"hits": {
				"total": {"value": 1},
				"hits": [{
					"_score": 1.5,
					"_source": {
						"product_id":"prod-1",
						"name":"Laptop",
						"description":"Portable",
						"price":150000,
						"category_id":"cat-1",
						"seller_id":"seller-1",
						"images":["img-1"],
						"status":"active"
					}
				}]
			}
		}`), nil
	})

	repo := NewElasticsearchRepository("http://example.test", "products", client)
	items, total, err := repo.Search(context.Background(), SearchFilter{Query: "laptop", Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "prod-1", items[0].ProductID)
	assert.Equal(t, float32(1.5), items[0].Score)
}

func TestBuildSearchRequest_AppliesFilters(t *testing.T) {
	req := buildSearchRequest(SearchFilter{
		Query:      "phone",
		CategoryID: "cat-1",
		PriceMin:   1000,
		PriceMax:   5000,
		SortBy:     "price_desc",
		Page:       2,
		PageSize:   10,
	})

	assert.Equal(t, 10, req["from"])
	assert.Equal(t, 10, req["size"])
	assert.NotNil(t, req["sort"])

	query := req["query"].(map[string]any)
	boolQuery := query["bool"].(map[string]any)
	assert.NotEmpty(t, boolQuery["must"])
	assert.Len(t, boolQuery["filter"], 3)
}
