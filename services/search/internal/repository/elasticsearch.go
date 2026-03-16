package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	apperrors "github.com/yourorg/micromart/pkg/errors"
	"github.com/yourorg/micromart/services/search/internal/model"
)

const productsIndexMapping = `{
  "mappings": {
    "properties": {
      "product_id": {"type": "keyword"},
      "name": {"type": "text", "fields": {"keyword": {"type": "keyword"}}},
      "description": {"type": "text"},
      "price": {"type": "long"},
      "category_id": {"type": "keyword"},
      "seller_id": {"type": "keyword"},
      "images": {"type": "keyword"},
      "status": {"type": "keyword"},
      "stock": {"type": "integer"}
    }
  }
}`

type elasticsearchRepository struct {
	baseURL    string
	indexName  string
	httpClient *http.Client
}

// NewElasticsearchRepository creates a repository backed by Elasticsearch HTTP APIs.
func NewElasticsearchRepository(baseURL, indexName string, httpClient *http.Client) SearchRepository {
	baseURL = strings.TrimRight(baseURL, "/")
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &elasticsearchRepository{
		baseURL:    baseURL,
		indexName:  indexName,
		httpClient: httpClient,
	}
}

func (r *elasticsearchRepository) Ping(ctx context.Context) error {
	resp, err := r.doRequest(ctx, http.MethodGet, "/", nil)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeUnavailable, err, "ping elasticsearch")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return apperrors.NewUnavailable("elasticsearch is unavailable")
	}
	return nil
}

func (r *elasticsearchRepository) EnsureIndex(ctx context.Context) error {
	resp, err := r.doRequest(ctx, http.MethodHead, "/"+r.indexName, nil)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeUnavailable, err, "check search index")
	}
	resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		createResp, err := r.doRequest(ctx, http.MethodPut, "/"+r.indexName, strings.NewReader(productsIndexMapping))
		if err != nil {
			return apperrors.Wrap(apperrors.CodeUnavailable, err, "create search index")
		}
		defer createResp.Body.Close()
		if createResp.StatusCode >= http.StatusBadRequest {
			body, _ := io.ReadAll(createResp.Body)
			return apperrors.NewInternal("create search index failed: %s", strings.TrimSpace(string(body)))
		}
		return nil
	default:
		body, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternal("check search index failed: %s", strings.TrimSpace(string(body)))
	}
}

func (r *elasticsearchRepository) Search(ctx context.Context, filter SearchFilter) ([]*model.ProductDocument, int64, error) {
	body, err := json.Marshal(buildSearchRequest(filter))
	if err != nil {
		return nil, 0, apperrors.Wrap(apperrors.CodeInternal, err, "marshal search request")
	}

	resp, err := r.doRequest(ctx, http.MethodPost, "/"+r.indexName+"/_search", bytes.NewReader(body))
	if err != nil {
		return nil, 0, apperrors.Wrap(apperrors.CodeUnavailable, err, "search products")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		raw, _ := io.ReadAll(resp.Body)
		return nil, 0, apperrors.NewUnavailable("search products failed: %s", strings.TrimSpace(string(raw)))
	}

	var payload struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Score  float32               `json:"_score"`
				Source model.ProductDocument `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, 0, apperrors.Wrap(apperrors.CodeInternal, err, "decode search response")
	}

	items := make([]*model.ProductDocument, 0, len(payload.Hits.Hits))
	for _, hit := range payload.Hits.Hits {
		doc := hit.Source
		doc.Score = hit.Score
		items = append(items, &doc)
	}
	return items, payload.Hits.Total.Value, nil
}

func (r *elasticsearchRepository) UpsertProduct(ctx context.Context, product *model.ProductDocument) error {
	body, err := json.Marshal(product)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, err, "marshal search document")
	}

	resp, err := r.doRequest(ctx, http.MethodPut, "/"+r.indexName+"/_doc/"+product.ProductID, bytes.NewReader(body))
	if err != nil {
		return apperrors.Wrap(apperrors.CodeUnavailable, err, "upsert search document")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		raw, _ := io.ReadAll(resp.Body)
		return apperrors.NewUnavailable("upsert search document failed: %s", strings.TrimSpace(string(raw)))
	}
	return nil
}

func (r *elasticsearchRepository) DeleteProduct(ctx context.Context, productID string) error {
	resp, err := r.doRequest(ctx, http.MethodDelete, "/"+r.indexName+"/_doc/"+productID, nil)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeUnavailable, err, "delete search document")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode >= http.StatusBadRequest {
		raw, _ := io.ReadAll(resp.Body)
		return apperrors.NewUnavailable("delete search document failed: %s", strings.TrimSpace(string(raw)))
	}
	return nil
}

func (r *elasticsearchRepository) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, r.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func buildSearchRequest(filter SearchFilter) map[string]any {
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	must := []any{}
	if strings.TrimSpace(filter.Query) != "" {
		must = append(must, map[string]any{
			"multi_match": map[string]any{
				"query":  filter.Query,
				"fields": []string{"name^3", "description"},
			},
		})
	}

	filters := []any{
		map[string]any{"term": map[string]any{"status": "active"}},
	}
	if filter.CategoryID != "" {
		filters = append(filters, map[string]any{"term": map[string]any{"category_id": filter.CategoryID}})
	}
	if filter.PriceMin > 0 || filter.PriceMax > 0 {
		rangeQuery := map[string]any{}
		if filter.PriceMin > 0 {
			rangeQuery["gte"] = filter.PriceMin
		}
		if filter.PriceMax > 0 {
			rangeQuery["lte"] = filter.PriceMax
		}
		filters = append(filters, map[string]any{"range": map[string]any{"price": rangeQuery}})
	}

	query := map[string]any{
		"bool": map[string]any{
			"filter": filters,
		},
	}
	if len(must) > 0 {
		query["bool"].(map[string]any)["must"] = must
	}

	req := map[string]any{
		"track_total_hits": true,
		"from":             int((page - 1) * pageSize),
		"size":             int(pageSize),
		"query":            query,
	}

	switch filter.SortBy {
	case "price_asc":
		req["sort"] = []any{map[string]any{"price": "asc"}}
	case "price_desc":
		req["sort"] = []any{map[string]any{"price": "desc"}}
	case "newest":
		req["sort"] = []any{map[string]any{"product_id": "desc"}}
	default:
		if len(must) == 0 {
			req["sort"] = []any{map[string]any{"product_id": "desc"}}
		}
	}

	return req
}
