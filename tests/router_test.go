package gobeansack_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/k0kubun/pp"
	"github.com/soumitsalman/beansapi/router"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	routeHealth        = "/health"
	routeTagCategories = "/tags/categories"
	routeTagEntities   = "/tags/entities"
	routeTagRegions    = "/tags/regions"
	routeSources       = "/sources"
	routeLatest        = "/articles/latest"
	routeTrending      = "/articles/trending"
	routeTopHeadlines  = "/articles/top-headlines"
	routeSearch        = "/articles/search"
	routePropagation   = "/articles/propagation"
)

func newTestHTTPServer(t *testing.T) *httptest.Server {
	t.Helper()
	db := setupTestDB()
	embedder := setupTestEmbedder()
	gin.SetMode(gin.TestMode)
	engine := router.NewRouter(db, embedder, nil, 0)
	srv := httptest.NewServer(engine)
	t.Cleanup(func() {
		srv.Close()
		embedder.Close()
		db.Close()
	})
	return srv
}

func routerURL(base, path string, params url.Values) string {
	raw := strings.TrimSuffix(base, "/") + path
	if enc := params.Encode(); enc != "" {
		raw += "?" + enc
	}
	return raw
}

func routerGET(t *testing.T, base, path string, params url.Values) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, routerURL(base, path, params), nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, body
}

func routerPOST(t *testing.T, base, path string, payload any) (int, []byte) {
	t.Helper()
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, strings.TrimSuffix(base, "/")+path, bytes.NewReader(data))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, body
}

func requireStatus(t *testing.T, expected, actual int, body []byte) {
	t.Helper()
	if len(bytes.TrimSpace(body)) > 0 {
		parseJSONValue(t, body)
	}
	require.Equal(t, expected, actual, "response body: %s", string(body))
}

func parseJSONValue(t *testing.T, body []byte) any {
	t.Helper()
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil
	}
	var v any
	err := json.Unmarshal(trimmed, &v)
	require.NoError(t, err, "response body is not valid JSON: %s", string(body))
	return v
}

func printResponse(t *testing.T, label string, body []byte) {
	t.Helper()
	if os.Getenv("TEST_PRINT_RESPONSE") == "" && !testing.Verbose() {
		return
	}
	pp.Println(label, parseJSONValue(t, body))
}

func parseStringArray(t *testing.T, body []byte) []string {
	t.Helper()
	v := parseJSONValue(t, body)
	arr, ok := v.([]any)
	require.True(t, ok, "expected JSON array of strings, got %T", v)
	out := make([]string, len(arr))
	for i, item := range arr {
		s, ok := item.(string)
		require.True(t, ok, "expected string at index %d, got %T", i, item)
		out[i] = s
	}
	return out
}

func parseJSONArray(t *testing.T, body []byte) []map[string]any {
	t.Helper()
	v := parseJSONValue(t, body)
	arr, ok := v.([]any)
	require.True(t, ok, "expected JSON array of objects, got %T", v)
	out := make([]map[string]any, len(arr))
	for i, item := range arr {
		obj, ok := item.(map[string]any)
		require.True(t, ok, "expected object at index %d, got %T", i, item)
		out[i] = obj
	}
	return out
}

func addArticleFilters(params url.Values) {
	params.Set("categories", testCategories[0])
	params.Set("limit", "5")
}

func skipIfEmbedderUnavailable(t *testing.T, status int, body []byte) {
	t.Helper()
	if status == http.StatusInternalServerError && strings.Contains(string(body), "Embedder just died") {
		t.Skip("embedder unavailable:", string(body))
	}
}

func requireOKOrNoContent(t *testing.T, status int, body []byte) {
	t.Helper()
	require.Contains(t, []int{http.StatusOK, http.StatusNoContent}, status, "response body: %s", string(body))
	if status == http.StatusOK {
		parseJSONValue(t, body)
	}
}

func repeatURLs(n int) []string {
	urls := make([]string, n)
	for i := range urls {
		urls[i] = fmt.Sprintf("https://example.com/article-%d", i)
	}
	return urls
}

func TestRouterHealth(t *testing.T) {
	srv := newTestHTTPServer(t)
	status, body := routerGET(t, srv.URL, routeHealth, nil)
	printResponse(t, "HEALTH", body)
	requireStatus(t, http.StatusOK, status, body)
}

func TestRouterGetTagCategories(t *testing.T) {
	srv := newTestHTTPServer(t)
	params := url.Values{}
	params.Set("limit", "5")
	params.Set("offset", "0")

	status, body := routerGET(t, srv.URL, routeTagCategories, params)
	printResponse(t, "TAG_CATEGORIES", body)
	requireStatus(t, http.StatusOK, status, body)
	tags := parseStringArray(t, body)
	assert.NotEmpty(t, tags)
}

func TestRouterGetTagEntities(t *testing.T) {
	srv := newTestHTTPServer(t)
	params := url.Values{}
	params.Set("limit", "5")

	status, body := routerGET(t, srv.URL, routeTagEntities, params)
	printResponse(t, "TAG_ENTITIES", body)
	requireStatus(t, http.StatusOK, status, body)
	entities := parseStringArray(t, body)
	assert.NotEmpty(t, entities)
}

func TestRouterGetTagRegions(t *testing.T) {
	srv := newTestHTTPServer(t)
	params := url.Values{}
	params.Set("limit", "5")

	status, body := routerGET(t, srv.URL, routeTagRegions, params)
	printResponse(t, "TAG_REGIONS", body)
	requireStatus(t, http.StatusOK, status, body)
	regions := parseStringArray(t, body)
	assert.NotEmpty(t, regions)
}

func TestRouterTagsInvalidLimit(t *testing.T) {
	srv := newTestHTTPServer(t)
	params := url.Values{}
	params.Set("limit", "0")
	status, body := routerGET(t, srv.URL, routeTagCategories, params)
	printResponse(t, "TAG_CATEGORIES_INVALID_LIMIT", body)
	requireStatus(t, http.StatusBadRequest, status, body)
}

func TestRouterTagsInvalidLimitHigh(t *testing.T) {
	srv := newTestHTTPServer(t)
	params := url.Values{}
	params.Set("limit", "999")
	status, body := routerGET(t, srv.URL, routeTagCategories, params)
	printResponse(t, "TAG_CATEGORIES_INVALID_LIMIT_HIGH", body)
	requireStatus(t, http.StatusBadRequest, status, body)
}

func TestRouterGetSources(t *testing.T) {
	srv := newTestHTTPServer(t)
	params := url.Values{}
	params.Set("sources", testSources[0])
	params.Set("limit", "5")

	status, body := routerGET(t, srv.URL, routeSources, params)
	printResponse(t, "SOURCES", body)
	requireStatus(t, http.StatusOK, status, body)
	items := parseJSONArray(t, body)
	assert.NotEmpty(t, items)
}

func TestRouterGetLatestArticles(t *testing.T) {
	srv := newTestHTTPServer(t)
	params := url.Values{}
	addArticleFilters(params)

	status, body := routerGET(t, srv.URL, routeLatest, params)
	printResponse(t, "LATEST_ARTICLES", body)
	requireOKOrNoContent(t, status, body)
	if status == http.StatusOK {
		items := parseJSONArray(t, body)
		assert.NotEmpty(t, items)
	}
}

func TestRouterGetTrendingArticles(t *testing.T) {
	srv := newTestHTTPServer(t)
	params := url.Values{}
	addArticleFilters(params)

	status, body := routerGET(t, srv.URL, routeTrending, params)
	printResponse(t, "TRENDING_ARTICLES", body)
	if status == http.StatusInternalServerError && strings.Contains(string(body), "DB just died") {
		t.Skip("trending query failed against live DB:", string(body))
	}
	requireOKOrNoContent(t, status, body)
	if status == http.StatusOK {
		items := parseJSONArray(t, body)
		assert.NotEmpty(t, items)
	}
}

func TestRouterGetTopHeadlines(t *testing.T) {
	srv := newTestHTTPServer(t)
	params := url.Values{}
	addArticleFilters(params)

	status, body := routerGET(t, srv.URL, routeTopHeadlines, params)
	printResponse(t, "TOP_HEADLINES", body)
	if status == http.StatusInternalServerError && strings.Contains(string(body), "DB just died") {
		t.Skip("top-headlines query failed against live DB:", string(body))
	}
	requireOKOrNoContent(t, status, body)
	if status == http.StatusOK {
		items := parseJSONArray(t, body)
		assert.NotEmpty(t, items)
	}
}

func TestRouterSearchArticlesMissingParam(t *testing.T) {
	srv := newTestHTTPServer(t)
	params := url.Values{}
	params.Set("limit", "5")

	status, body := routerGET(t, srv.URL, routeSearch, params)
	printResponse(t, "SEARCH_MISSING_PARAM", body)
	requireStatus(t, http.StatusBadRequest, status, body)
}

func TestRouterSearchArticlesByCategories(t *testing.T) {
	srv := newTestHTTPServer(t)
	params := url.Values{}
	params.Set("categories", testCategories[0])
	params.Set("limit", "5")

	status, body := routerGET(t, srv.URL, routeSearch, params)
	printResponse(t, "SEARCH_BY_CATEGORIES", body)
	requireStatus(t, http.StatusOK, status, body)
	items := parseJSONArray(t, body)
	assert.NotEmpty(t, items)
}

func TestRouterVectorSearchArticles(t *testing.T) {
	srv := newTestHTTPServer(t)
	params := url.Values{}
	params.Set("q", testVectorQuery)
	params.Set("acc", "0.6")
	params.Set("limit", "5")

	status, body := routerGET(t, srv.URL, routeSearch, params)
	printResponse(t, "VECTOR_SEARCH", body)
	skipIfEmbedderUnavailable(t, status, body)
	requireStatus(t, http.StatusOK, status, body)
	items := parseJSONArray(t, body)
	assert.NotEmpty(t, items)
}

func TestRouterVectorSearchLatest(t *testing.T) {
	srv := newTestHTTPServer(t)
	params := url.Values{}
	params.Set("q", testVectorQuery)
	params.Set("acc", "0.6")
	params.Set("limit", "5")

	status, body := routerGET(t, srv.URL, routeLatest, params)
	printResponse(t, "VECTOR_SEARCH_LATEST", body)
	skipIfEmbedderUnavailable(t, status, body)
	requireOKOrNoContent(t, status, body)
	if status == http.StatusOK {
		items := parseJSONArray(t, body)
		assert.NotEmpty(t, items)
	}
}

func TestRouterPropagationGETValid(t *testing.T) {
	srv := newTestHTTPServer(t)
	params := url.Values{}
	params.Set("urls", strings.Join(testPropagationURLs, ","))

	status, body := routerGET(t, srv.URL, routePropagation, params)
	printResponse(t, "PROPAGATION_GET", body)
	requireStatus(t, http.StatusOK, status, body)

	results := parseJSONArray(t, body)
	assert.Len(t, results, len(testPropagationURLs))
	for i, r := range results {
		assert.Equal(t, testPropagationURLs[i], r["url"])
	}
}

func TestRouterPropagationPOSTMaxURLs(t *testing.T) {
	srv := newTestHTTPServer(t)
	status, body := routerPOST(t, srv.URL, routePropagation, map[string]any{
		"urls": repeatURLs(129),
	})
	printResponse(t, "PROPAGATION_POST_MAX_URLS", body)
	requireStatus(t, http.StatusBadRequest, status, body)
}

func TestRouterPropagationGETMaxURLs(t *testing.T) {
	srv := newTestHTTPServer(t)
	params := url.Values{}
	params.Set("urls", strings.Join(repeatURLs(129), ","))
	status, body := routerGET(t, srv.URL, routePropagation, params)
	printResponse(t, "PROPAGATION_GET_MAX_URLS", body)
	requireStatus(t, http.StatusBadRequest, status, body)
}

func TestRouterPropagationPOSTValid(t *testing.T) {
	srv := newTestHTTPServer(t)
	status, body := routerPOST(t, srv.URL, routePropagation, map[string]any{"urls": testPropagationURLs})
	printResponse(t, "PROPAGATION_POST", body)
	requireStatus(t, http.StatusOK, status, body)

	results := parseJSONArray(t, body)
	assert.Len(t, results, len(testPropagationURLs))
	for i, r := range results {
		assert.Equal(t, testPropagationURLs[i], r["url"])
	}
}
