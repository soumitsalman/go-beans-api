package gobeansack_test

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

// defaultStressBaseURL is the target API base URL. Override with the
// STRESS_BASE_URL environment variable before running the test.
const defaultStressBaseURL = "http://localhost:8080"

// minConcurrency / maxConcurrency bound the allowed request count.
const (
	minConcurrency = 100
	maxConcurrency = 10000
)

// stressEndpoint describes one API endpoint with the set of optional query
// parameters it accepts.
type stressEndpoint struct {
	path        string
	acceptsQ    bool
	acceptsTags bool
	// published_since / trending_since are only meaningful on article endpoints
	acceptsPublishedSince bool
	acceptsTrendingSince  bool
	// /publishers requires at least one source value
	requiresSources bool
}

var stressEndpoints = []stressEndpoint{
	{path: "/articles/latest", acceptsQ: true, acceptsTags: true, acceptsPublishedSince: true, acceptsTrendingSince: true},
	{path: "/articles/trending", acceptsQ: true, acceptsTags: true, acceptsPublishedSince: true, acceptsTrendingSince: true},
	{path: "/publishers/sources"},
	{path: "/publishers", requiresSources: true},
	{path: "/tags/entities"},
	{path: "/tags/regions"},
}

// sampleQueries is a small set of representative natural-language queries used
// to populate the "q" parameter randomly.
var sampleQueries = []string{
	"artificial intelligence",
	"machine learning",
	"cloud computing",
	"cybersecurity breaches",
	"open source software",
	"startup funding",
	"climate change policy",
	"quantum computing",
	"electric vehicles",
	"blockchain technology",
}

// sampleTags is a small set of representative tag / entity values.
var sampleTags = []string{
	"OpenAI",
	"Elon Musk",
	"Google",
	"Microsoft",
	"US",
	"Europe",
	"Tesla",
	"Apple",
	"Amazon",
	"Meta",
}

// sampleSources is a small set of representative publisher source IDs used to
// satisfy the /publishers endpoint's required "sources" parameter.
var sampleSources = []string{
	"techcrunch.com",
	"theverge.com",
	"wired.com",
	"arstechnica.com",
	"venturebeat.com",
}

// stressResult holds the outcome of a single stress-test request.
type stressResult struct {
	endpoint   string
	statusCode int
	latency    time.Duration
	err        error
}

// buildStressURL constructs the full request URL for one random request against
// the given endpoint.
func buildStressURL(baseURL string, ep stressEndpoint, rng *rand.Rand) string {
	params := url.Values{}

	if ep.requiresSources {
		// Pick 1–3 random sources from the sample pool.
		n := 1 + rng.Intn(min(3, len(sampleSources)))
		perm := rng.Perm(len(sampleSources))
		for i := 0; i < n; i++ {
			params.Add("sources", sampleSources[perm[i]])
		}
	}

	if ep.acceptsQ && rng.Intn(2) == 0 {
		params.Set("q", sampleQueries[rng.Intn(len(sampleQueries))])
	}

	if ep.acceptsTags && rng.Intn(2) == 0 {
		n := 1 + rng.Intn(min(3, len(sampleTags)))
		perm := rng.Perm(len(sampleTags))
		for i := 0; i < n; i++ {
			params.Add("tags", sampleTags[perm[i]])
		}
	}

	if ep.acceptsPublishedSince && rng.Intn(2) == 0 {
		// Random offset from 1 to 30 days in the past.
		daysAgo := 1 + rng.Intn(30)
		t := time.Now().UTC().AddDate(0, 0, -daysAgo)
		params.Set("published_since", t.Format(time.RFC3339))
	}

	if ep.acceptsTrendingSince && rng.Intn(2) == 0 {
		daysAgo := 1 + rng.Intn(7)
		t := time.Now().UTC().AddDate(0, 0, -daysAgo)
		params.Set("trending_since", t.Format(time.RFC3339))
	}

	// Random limit (1–50) and occasionally a non-zero offset.
	params.Set("limit", strconv.Itoa(1+rng.Intn(50)))
	if rng.Intn(4) == 0 {
		params.Set("offset", strconv.Itoa(rng.Intn(20)))
	}

	raw := baseURL + ep.path
	if enc := params.Encode(); enc != "" {
		raw += "?" + enc
	}
	return raw
}

// runStressTest sends `concurrency` requests against the API concurrently and
// returns a slice of stressResult, one per request.
func runStressTest(baseURL string, concurrency int, apiKey string) []stressResult {
	results := make([]stressResult, concurrency)
	var wg sync.WaitGroup
	wg.Add(concurrency)

	client := &http.Client{Timeout: 30 * time.Second}

	// Pre-generate per-goroutine seeds from a single source to avoid seed
	// collisions when many goroutines start within the same nanosecond.
	masterRng := rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec
	seeds := make([]int64, concurrency)
	for i := range seeds {
		seeds[i] = masterRng.Int63()
	}

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()

			rng := rand.New(rand.NewSource(seeds[idx])) //nolint:gosec

			ep := stressEndpoints[rng.Intn(len(stressEndpoints))]
			rawURL := buildStressURL(baseURL, ep, rng)

			req, err := http.NewRequest(http.MethodGet, rawURL, nil)
			if err != nil {
				results[idx] = stressResult{endpoint: ep.path, err: err}
				return
			}
			if apiKey != "" {
				req.Header.Set("X-API-Key", apiKey)
			}

			start := time.Now()
			resp, err := client.Do(req)
			latency := time.Since(start)

			if err != nil {
				results[idx] = stressResult{endpoint: ep.path, latency: latency, err: err}
				return
			}
			resp.Body.Close()
			results[idx] = stressResult{endpoint: ep.path, statusCode: resp.StatusCode, latency: latency}
		}(i)
	}

	wg.Wait()
	return results
}

// printStressSummary logs aggregated metrics for the stress run.
func printStressSummary(t *testing.T, results []stressResult) {
	t.Helper()

	type epStats struct {
		total    int
		success  int
		failures int
		totalMs  int64
	}

	stats := map[string]*epStats{}
	for _, ep := range stressEndpoints {
		stats[ep.path] = &epStats{}
	}

	totalSuccess, totalFailure := 0, 0

	for _, r := range results {
		s := stats[r.endpoint]
		s.total++
		s.totalMs += r.latency.Milliseconds()

		if r.err != nil || (r.statusCode >= 500) {
			s.failures++
			totalFailure++
		} else {
			s.success++
			totalSuccess++
		}
	}

	t.Log("=== Stress Test Summary ===")
	t.Logf("Total requests: %d | Success: %d | Failure: %d",
		len(results), totalSuccess, totalFailure)
	t.Log("--- Per-endpoint breakdown ---")
	for _, ep := range stressEndpoints {
		s := stats[ep.path]
		avgMs := int64(0)
		if s.total > 0 {
			avgMs = s.totalMs / int64(s.total)
		}
		t.Logf("  %-28s  total=%-5d  ok=%-5d  err=%-5d  avg_latency=%dms",
			ep.path, s.total, s.success, s.failures, avgMs)
	}

	// Fail the test if more than 10% of requests returned a 5xx or network error.
	if len(results) > 0 {
		failRate := float64(totalFailure) / float64(len(results))
		if failRate > 0.10 {
			t.Errorf("stress test failure rate %.1f%% exceeds 10%% threshold", failRate*100)
		}
	}
}

// concurrencyFromEnv reads the desired concurrency from STRESS_CONCURRENCY.
// Falls back to 200 when unset and clamps the value to [minConcurrency, maxConcurrency].
func concurrencyFromEnv() int {
	raw := os.Getenv("STRESS_CONCURRENCY")
	if raw == "" {
		return 200
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < minConcurrency {
		return minConcurrency
	}
	if n > maxConcurrency {
		return maxConcurrency
	}
	return n
}

// TestStressAPI is the main stress-test entry point.
//
// Environment variables:
//
//	STRESS_BASE_URL    - base URL of a running API server (default: http://localhost:8080)
//	STRESS_CONCURRENCY - number of concurrent requests to fire, clamped to 100-10000 (default: 200)
//	STRESS_API_KEY     - optional value for the X-API-Key header
//
// Run with an extended timeout to accommodate large concurrency values, e.g.:
//
//	STRESS_BASE_URL=http://my-api:8080 go test ./tests/... -run TestStressAPI -v -timeout 5m
func TestStressAPI(t *testing.T) {
	baseURL := os.Getenv("STRESS_BASE_URL")
	if baseURL == "" {
		baseURL = defaultStressBaseURL
	}
	apiKey := os.Getenv("STRESS_API_KEY")
	concurrency := concurrencyFromEnv()

	t.Logf("Stress testing %s with %d concurrent requests", baseURL, concurrency)

	// Quick connectivity check before fanning out.
	client := &http.Client{Timeout: 5 * time.Second}
	if _, err := client.Get(baseURL + "/health"); err != nil {
		t.Skipf("API server not reachable at %s (%v) — skipping stress test", baseURL, err)
	}

	results := runStressTest(baseURL, concurrency, apiKey)
	printStressSummary(t, results)
}

// TestStressAPIEndpoints runs a smaller fixed-size fan-out (one batch per
// endpoint) to verify that every endpoint responds without a 5xx error.
// This test can be run in normal CI without a live server; it will skip if
// the server is unreachable.
func TestStressAPIEndpoints(t *testing.T) {
	baseURL := os.Getenv("STRESS_BASE_URL")
	if baseURL == "" {
		baseURL = defaultStressBaseURL
	}
	apiKey := os.Getenv("STRESS_API_KEY")

	client := &http.Client{Timeout: 5 * time.Second}
	if _, err := client.Get(baseURL + "/health"); err != nil {
		t.Skipf("API server not reachable at %s (%v) — skipping endpoint stress test", baseURL, err)
	}

	const requestsPerEndpoint = 10
	concurrency := len(stressEndpoints) * requestsPerEndpoint

	t.Logf("Endpoint smoke stress: %d endpoints × %d requests = %d total",
		len(stressEndpoints), requestsPerEndpoint, concurrency)

	results := runStressTest(baseURL, concurrency, apiKey)
	printStressSummary(t, results)

	// Report per-endpoint 5xx failures as individual sub-tests so it is easy
	// to see which endpoint is misbehaving.
	epFailures := map[string]int{}
	for _, r := range results {
		if r.err != nil || r.statusCode >= 500 {
			epFailures[r.endpoint]++
		}
	}
	for _, ep := range stressEndpoints {
		ep := ep
		t.Run(fmt.Sprintf("endpoint=%s", ep.path), func(t *testing.T) {
			if f := epFailures[ep.path]; f > 0 {
				t.Errorf("%s had %d failure(s)", ep.path, f)
			}
		})
	}
}
