package router

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/maypok86/otter/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	bs "github.com/soumitsalman/beansapi/beansack"
	"github.com/soumitsalman/beansapi/nlp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

const (
	MIN_WINDOW          = 1
	DEFAULT_WINDOW      = 7 // DAYS
	DEFAULT_ACCURACY    = 0.75
	DEFAULT_LIMIT       = 16
	MAX_LIMIT           = 128
	FAVICON_PATH        = "./images/beans.png"
	DEFAULT_CONCURRENCY = 512
)

const (
	_CACHE_SIZE = 1000
	_CACHE_TTL  = 30 * time.Minute
)

const (
	_EMBEDDER_ERROR     = "Embedder just died. Retry in a bit."
	_DB_ERROR           = "DB just died. Retry in a bit."
	_NEEDS_SEARCH_PARAM = "At least one search parameter is required (q, tags, categories, regions, entities)."
)

const (
	_BEAN_TREND_FIELDS = "likes, comments, shares, related, trend_score"
)

// PaginationInput holds shared list-endpoint pagination query params.
// form default=16 and max=128 must stay in sync with DEFAULT_LIMIT and MAX_LIMIT.
type PaginationInput struct {
	Limit  int `form:"limit,default=16" binding:"min=1,max=128"`
	Offset int `form:"offset" binding:"min=0"`
}

func (p PaginationInput) ToDB() bs.Pagination {
	return bs.Pagination{Limit: p.Limit, Offset: p.Offset}
}

func normalizePagination(p *PaginationInput) error {
	if p.Limit == 0 {
		p.Limit = DEFAULT_LIMIT
	}
	if p.Limit > MAX_LIMIT {
		return fmt.Errorf("limit must be <= %d", MAX_LIMIT)
	}
	if p.Offset < 0 {
		return fmt.Errorf("offset must be >= 0")
	}
	return nil
}

// TagsInput describes pagination for tag discovery endpoints (/tags/*).
type TagsInput struct {
	PaginationInput
}

// PublishersInput describes query parameters for getPublishers (/sources).
type PublishersInput struct {
	// Sources: Publisher source IDs to resolve (CSV). Required for meaningful results.
	Sources []string `form:"sources" collection_format:"csv"`
	PaginationInput
}

// ArticlesInput describes shared query parameters for article list and search endpoints.
type ArticlesInput struct {
	// URLs: Optional list of article URLs to fetch directly (CSV).
	URLs []string `form:"urls" collection_format:"csv"`
	// Q: Free-text semantic/vector search query (max 512 chars).
	Q string `form:"q" binding:"max=512"`
	// Acc: Similarity accuracy threshold (0.0-1.0). Higher => stricter match.
	// Used to compute vector distance (distance = 1 - Acc).
	Acc float64 `form:"acc,default=0.75" binding:"min=0,max=1"`
	// ContentType: Optional content type filter (e.g., "news" or "blog").
	ContentType string `form:"content_type" binding:"omitempty,oneof=news blog"`
	// Categories: Filter results to one or more categories/topics (CSV).
	Categories []string `form:"categories" collection_format:"csv"`
	// Regions: Filter results to one or more geographic regions (CSV).
	Regions []string `form:"regions" collection_format:"csv"`
	// Entities: Filter results to one or more named entities (CSV).
	Entities []string `form:"entities" collection_format:"csv"`
	// Tags: Tag/keyword filters (CSV). Combined into a full-text query for tag matching.
	Tags []string `form:"tags" collection_format:"csv"`
	// Sources: Publisher/source IDs to include (CSV).
	Sources []string `form:"sources" collection_format:"csv"`
	// From: Start date for published/updated filtering (format YYYY-MM-DD).
	From time.Time `form:"from" time_format:"2006-01-02" swaggertype:"string" format:"date"`
	// FullContent: If true, include full article content in results (larger payload).
	FullContent bool `form:"full_content,default=false"`
	// PaginationInput: Embeds common pagination params (Limit, Offset).
	PaginationInput
}

type PropagationInput struct {
	// URLs lists seed article URLs to analyze for cross-outlet coverage and social mentions (1–128 items).
	URLs []string `form:"urls" json:"urls" collection_format:"csv" binding:"required,min=1,dive,url"`
}

func bindPropagationInput(c *gin.Context) (PropagationInput, error) {
	var input PropagationInput
	switch c.Request.Method {
	case http.MethodGet:
		if err := c.ShouldBindQuery(&input); err != nil {
			return PropagationInput{}, err
		}
	case http.MethodPost:
		if err := c.ShouldBindJSON(&input); err != nil {
			return PropagationInput{}, err
		}
	default:
		return PropagationInput{}, fmt.Errorf("method not allowed")
	}
	if len(input.URLs) > MAX_LIMIT {
		return PropagationInput{}, fmt.Errorf("urls must contain at most %d items", MAX_LIMIT)
	}
	return input, nil
}

type Configuration struct {
	DB       bs.Beansack
	Embedder nlp.Embedder
	APIKeys  map[string]string
	queue    chan int
	cache    *otter.Cache[string, []float32]
}

// health godoc
// @Summary Check API health
// @Description Lightweight liveness probe. Use before other tools to confirm the service is reachable. No authentication required when API keys are disabled.
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]string "status alive"
// @ID healthCheck
// @Router /health [get]
func (r *Configuration) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "alive"})
}

func validateTagsParams(c *gin.Context) {
	var input TagsInput
	if err := c.ShouldBindQuery(&input); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := normalizePagination(&input.PaginationInput); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Set("req_params", input)
	c.Set("req_page", input.PaginationInput.ToDB())
	c.Next()
}

// getCategories godoc
// @Summary List article category topics
// @Description Discover valid values for the `categories` filter on article endpoints.
// @Description Returns a paginated array of unique topic labels extracted from indexed articles (e.g. "Artificial Intelligence", "Cybersecurity", "Politics").
// @Description **When to use**: call before searchArticles or feed endpoints when you need exact, case-sensitive category strings.
// @Description **Related tools**: listEntities, listRegions, searchArticles.
// @Description **Pagination**: use `offset` to walk the full catalog when `limit` < total count.
// @Tags Tags
// @Produce json
// @Param limit query int false "Page size. Default 16, max 128." default(16) minimum(1) maximum(128)
// @Param offset query int false "Number of items to skip. Default 0." minimum(0)
// @Success 200 {array} string "JSON array of category label strings"
// @Success 204 "No categories in index (empty result, not an error)"
// @Failure 400 {object} map[string]string "Invalid limit or offset"
// @Failure 401 {object} map[string]string "Missing or invalid API key"
// @Failure 429 {object} map[string]string "Concurrency limit exceeded; retry shortly"
// @Failure 500 {object} map[string]string "Database unavailable; retry"
// @ID listCategories
// @Router /tags/categories [get]
func (r *Configuration) getCategories(c *gin.Context) {
	page := c.MustGet("req_page").(bs.Pagination)
	data, err := r.DB.DistinctCategories(c.Request.Context(), page)
	returnResponse(c, data, err)
}

// getEntities godoc
// @Summary List named entities
// @Description Discover valid values for the `entities` filter on article endpoints.
// @Description Returns a paginated array of unique named entities (people, organizations, products, places) extracted via NLP from article text.
// @Description **When to use**: call before searchArticles when filtering by specific people, companies, or places with exact spelling.
// @Description **Related tools**: listCategories, listRegions, searchArticles.
// @Tags Tags
// @Produce json
// @Param limit query int false "Page size. Default 16, max 128." default(16) minimum(1) maximum(128)
// @Param offset query int false "Number of items to skip. Default 0." minimum(0)
// @Success 200 {array} string "JSON array of entity label strings"
// @Success 204 "No entities in index (empty result, not an error)"
// @Failure 400 {object} map[string]string "Invalid limit or offset"
// @Failure 401 {object} map[string]string "Missing or invalid API key"
// @Failure 429 {object} map[string]string "Concurrency limit exceeded; retry shortly"
// @Failure 500 {object} map[string]string "Database unavailable; retry"
// @ID listEntities
// @Router /tags/entities [get]
func (r *Configuration) getEntities(c *gin.Context) {
	page := c.MustGet("req_page").(bs.Pagination)
	data, err := r.DB.DistinctEntities(c.Request.Context(), page)
	returnResponse(c, data, err)
}

// getRegions godoc
// @Summary List geographic regions
// @Description Discover valid values for the `regions` filter on article endpoints.
// @Description Returns a paginated array of unique geographic region labels (e.g. "North America", "Europe", "India", "Middle East").
// @Description **When to use**: call before searchArticles when filtering by geography with exact, case-sensitive region strings.
// @Description **Related tools**: listCategories, listEntities, searchArticles.
// @Tags Tags
// @Produce json
// @Param limit query int false "Page size. Default 16, max 128." default(16) minimum(1) maximum(128)
// @Param offset query int false "Number of items to skip. Default 0." minimum(0)
// @Success 200 {array} string "JSON array of region label strings"
// @Success 204 "No regions in index (empty result, not an error)"
// @Failure 400 {object} map[string]string "Invalid limit or offset"
// @Failure 401 {object} map[string]string "Missing or invalid API key"
// @Failure 429 {object} map[string]string "Concurrency limit exceeded; retry shortly"
// @Failure 500 {object} map[string]string "Database unavailable; retry"
// @ID listRegions
// @Router /tags/regions [get]
func (r *Configuration) getRegions(c *gin.Context) {
	page := c.MustGet("req_page").(bs.Pagination)
	data, err := r.DB.DistinctRegions(c.Request.Context(), page)
	returnResponse(c, data, err)
}

func validatePublishersParams(c *gin.Context) {
	var input PublishersInput
	if err := c.ShouldBindQuery(&input); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := normalizePagination(&input.PaginationInput); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Set("req_params", input)
	c.Set("req_conditions", bs.Condition{Sources: input.Sources})
	c.Set("req_page", input.PaginationInput.ToDB())
	c.Next()
}

// getPublishers godoc
// @Summary Resolve publisher source metadata
// @Description Look up display metadata for one or more publisher source IDs found on article `source` fields.
// @Description Returns site name, base URL, description, and favicon for each requested source ID.
// @Description **When to use**: after searchArticles or feed endpoints to humanize source IDs in UI or agent responses.
// @Description **Required**: at least one value in `sources` (comma-separated source IDs, e.g. techcrunch.com).
// @Description **Related tools**: searchArticles, getLatestArticles.
// @Tags Publishers
// @Produce json
// @Param sources query []string true "Publisher source IDs to resolve (CSV). Example: techcrunch.com,theverge.com" collectionFormat(csv)
// @Param limit query int false "Page size. Default 16, max 128." default(16) minimum(1) maximum(128)
// @Param offset query int false "Number of items to skip. Default 0." minimum(0)
// @Success 200 {array} beansack.Publisher "Publisher metadata objects keyed by source ID"
// @Success 204 "No matching publishers (empty result, not an error)"
// @Failure 400 {object} map[string]string "Missing sources or invalid pagination"
// @Failure 401 {object} map[string]string "Missing or invalid API key"
// @Failure 429 {object} map[string]string "Concurrency limit exceeded; retry shortly"
// @Failure 500 {object} map[string]string "Database unavailable; retry"
// @ID getPublishers
// @Router /sources [get]
func (r *Configuration) getPublishers(c *gin.Context) {
	conditions := c.MustGet("req_conditions").(bs.Condition)
	page := c.MustGet("req_page").(bs.Pagination)
	items, err := r.DB.QueryPublishers(c.Request.Context(), conditions, page, []string{bs.CORE_PUBLISHER_FIELDS})
	returnResponse(c, items, err)
}

func (config *Configuration) validateArticlesParams(c *gin.Context) {
	var input ArticlesInput
	if err := c.ShouldBindQuery(&input); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := normalizePagination(&input.PaginationInput); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	conditions := bs.Condition{
		URLs:       input.URLs,
		Kind:       input.ContentType,
		Created:    input.From,
		Updated:    input.From,
		Tags:       input.Tags,
		Categories: input.Categories,
		Regions:    input.Regions,
		Entities:   input.Entities,
		Sources:    input.Sources,
		Extra:      []string{},
	}
	if input.FullContent {
		conditions.Extra = append(conditions.Extra, bs.UNRESTRICTED_CONTENT_CONDITIONS)
	}
	if input.Q != "" {
		conditions.Distance = 1 - input.Acc
		if embedding, found := config.cache.GetIfPresent(input.Q); found {
			conditions.Embedding = embedding
		} else {
			conditions.Embedding = config.Embedder.EmbedQuery(c, input.Q)
			if len(conditions.Embedding) == 0 {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": _EMBEDDER_ERROR})
				return
			}
			config.cache.Set(input.Q, conditions.Embedding)
		}
	}
	columns := []string{bs.CORE_BEAN_FIELDS}
	if input.FullContent {
		columns = append(columns, bs.K_CONTENT)
	}
	c.Set("req_params", input)
	c.Set("req_conditions", conditions)
	c.Set("req_page", input.PaginationInput.ToDB())
	c.Set("req_columns", columns)
	c.Next()
}

// searchArticles godoc
// @Summary Search all articles by topic, tags, or URL
// @Description **Primary MCP tool** — full-corpus search sorted by relevance.
// @Description **Requires at least one of**: `q`, `tags`, `categories`, `regions`, `entities`, or `urls`.
// @Description **Search modes** (combinable with filters):
// @Description - `q` + `acc`: semantic vector search over article embeddings (natural language, 3–512 chars).
// @Description - `tags`: fuzzy text match across categories, regions, and entities (AND between tag values; case/whitespace insensitive).
// @Description - `categories` / `regions` / `entities`: exact array filters (OR within each dimension; case/whitespace sensitive — discover values via listCategories, listEntities, listRegions).
// @Description - `urls`: fetch specific articles by canonical URL (CSV).
// @Description **Performance**: scans the full index; prefer `full_content=false` unless the body is needed. Heavier than feed endpoints.
// @Description **Related tools**: listCategories, listEntities, listRegions, getPublishers, getArticlePropagation.
// @Tags Articles
// @Accept json
// @Produce json
// @Param q query string false "Semantic search query in natural language (3–512 chars). Ranks by embedding similarity."
// @Param acc query number false "Match strictness when q is set. 0.0=broad, 1.0=strict. Default 0.75." default(0.75) minimum(0) maximum(1)
// @Param content_type query string false "Restrict to content kind: news or blog." Enums(news,blog)
// @Param urls query []string false "Fetch articles by exact URL (CSV). Satisfies the required-search-param rule on its own." collectionFormat(csv)
// @Param tags query []string false "Fuzzy filter across categories+regions+entities (AND between values). Good when exact tag spelling is unknown." collectionFormat(csv)
// @Param categories query []string false "Exact topic filter (OR). Case sensitive — use listCategories first." collectionFormat(csv)
// @Param regions query []string false "Exact region filter (OR). Case sensitive — use listRegions first." collectionFormat(csv)
// @Param entities query []string false "Exact entity filter (OR). Case sensitive — use listEntities first." collectionFormat(csv)
// @Param sources query []string false "Publisher source ID filter (OR). Resolve names via getPublishers." collectionFormat(csv)
// @Param from query string false "Only articles published or updated on/after this date (YYYY-MM-DD)." format(date)
// @Param full_content query bool false "Include full article body. Default false (summary only)." default(false)
// @Param limit query int false "Page size. Default 16, max 128." default(16) minimum(1) maximum(128)
// @Param offset query int false "Skip N results for pagination. Default 0." minimum(0)
// @Success 200 {array} beansack.BeanAggregate "Articles with publisher info, engagement metrics, and trend_score"
// @Success 204 "No matching articles (empty result, not an error)"
// @Failure 400 {object} map[string]string "Missing required search param or invalid input"
// @Failure 401 {object} map[string]string "Missing or invalid API key"
// @Failure 429 {object} map[string]string "Concurrency limit exceeded; retry shortly"
// @Failure 500 {object} map[string]string "Database or embedder unavailable; retry"
// @ID searchArticles
// @Router /articles/search [get]
func (r *Configuration) searchArticles(c *gin.Context) {
	input := c.MustGet("req_params").(ArticlesInput)
	conditions := c.MustGet("req_conditions").(bs.Condition)
	page := c.MustGet("req_page").(bs.Pagination)
	// the precanned columns do not apply here
	columns := []string{bs.EXTENDED_BEAN_FIELDS}
	if input.FullContent {
		columns = append(columns, bs.K_CONTENT)
	}
	// NOTE: if no time window is given, thats fine
	// but it should at least provide some search param
	if (len(conditions.Embedding) |
		len(conditions.Tags) |
		len(conditions.Categories) |
		len(conditions.Regions) |
		len(conditions.Entities) |
		len(conditions.URLs)) == 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": _NEEDS_SEARCH_PARAM})
		return
	}
	// return all columns
	items, err := r.DB.QueryBeans(c.Request.Context(), conditions, page, columns)
	returnResponse(c, items, err)
}

// getLatestArticles godoc
// @Summary Search or list newest articles (reverse chronological)
// @Description Returns recently published articles sorted by publish date (newest first).
// @Description **Time window**: if `from` is omitted, defaults to the last 7 days.
// @Description **Filters** (all optional): same semantics as searchArticles — `q` for semantic search, `tags` for fuzzy match, or exact `categories`/`regions`/`entities`/`sources`.
// @Description **When to use**: monitoring recent news in a topic without full-corpus search cost. Lighter than searchArticles.
// @Description **Related tools**: listCategories, listEntities, listRegions, searchArticles, getTrendingArticles.
// @Tags Articles
// @Accept json
// @Produce json
// @Param q query string false "Optional semantic search query (3–512 chars). Narrows results by embedding similarity."
// @Param acc query number false "Match strictness when q is set. Default 0.75." default(0.75) minimum(0) maximum(1)
// @Param content_type query string false "Restrict to content kind: news or blog." Enums(news,blog)
// @Param tags query []string false "Fuzzy filter across categories+regions+entities (AND between values)." collectionFormat(csv)
// @Param categories query []string false "Exact topic filter (OR). Use listCategories for valid values." collectionFormat(csv)
// @Param regions query []string false "Exact region filter (OR). Use listRegions for valid values." collectionFormat(csv)
// @Param entities query []string false "Exact entity filter (OR). Use listEntities for valid values." collectionFormat(csv)
// @Param sources query []string false "Publisher source ID filter (OR)." collectionFormat(csv)
// @Param from query string false "Published on/after this date (YYYY-MM-DD). Defaults to 7 days ago when omitted." format(date)
// @Param full_content query bool false "Include full article body. Default false." default(false)
// @Param limit query int false "Page size. Default 16, max 128." default(16) minimum(1) maximum(128)
// @Param offset query int false "Skip N results. Default 0." minimum(0)
// @Success 200 {array} beansack.Bean "Latest articles sorted by published_at descending"
// @Success 204 "No articles in window (empty result, not an error)"
// @Failure 400 {object} map[string]string "Invalid query parameters"
// @Failure 401 {object} map[string]string "Missing or invalid API key"
// @Failure 429 {object} map[string]string "Concurrency limit exceeded; retry shortly"
// @Failure 500 {object} map[string]string "Database or embedder unavailable; retry"
// @ID getLatestArticles
// @Router /articles/latest [get]
func (r *Configuration) getLatestArticles(c *gin.Context) {
	conditions := c.MustGet("req_conditions").(bs.Condition)
	page := c.MustGet("req_page").(bs.Pagination)
	columns := c.MustGet("req_columns").([]string)
	// default to last 7 days if no date filter is there
	// and disable trending filter
	if conditions.Created.IsZero() {
		conditions.Created = time.Now().AddDate(0, 0, -DEFAULT_WINDOW) // default to last 7 days if no published/trending filter provided
	}
	conditions.Updated = time.Time{}
	items, err := r.DB.QueryLatestBeans(c.Request.Context(), conditions, page, columns)
	returnResponse(c, items, err)
}

// getTrendingArticles godoc
// @Summary Search or list trending articles by engagement score
// @Description Returns articles ranked by `trend_score` (highest first). Trend score blends social engagement (likes, comments, shares), cross-outlet coverage, and recency.
// @Description **Time window**: if `from` is omitted, defaults to the last 7 days of trending activity.
// @Description **Filters** (all optional): same semantics as searchArticles.
// @Description **When to use**: surface what is gaining traction now — prefer over getLatestArticles when popularity matters more than recency alone.
// @Description **Related tools**: getTopHeadlines (24h subset), searchArticles, getArticlePropagation.
// @Tags Articles
// @Accept json
// @Produce json
// @Param q query string false "Optional semantic search query (3–512 chars)."
// @Param acc query number false "Match strictness when q is set. Default 0.75." default(0.75) minimum(0) maximum(1)
// @Param content_type query string false "Restrict to content kind: news or blog." Enums(news,blog)
// @Param tags query []string false "Fuzzy filter across categories+regions+entities (AND between values)." collectionFormat(csv)
// @Param categories query []string false "Exact topic filter (OR)." collectionFormat(csv)
// @Param regions query []string false "Exact region filter (OR)." collectionFormat(csv)
// @Param entities query []string false "Exact entity filter (OR)." collectionFormat(csv)
// @Param sources query []string false "Publisher source ID filter (OR)." collectionFormat(csv)
// @Param from query string false "Trending activity since this date (YYYY-MM-DD). Defaults to 7 days ago when omitted." format(date)
// @Param full_content query bool false "Include full article body. Default false." default(false)
// @Param limit query int false "Page size. Default 16, max 128." default(16) minimum(1) maximum(128)
// @Param offset query int false "Skip N results. Default 0." minimum(0)
// @Success 200 {array} beansack.BeanTrend "Articles with engagement metrics and trend_score, sorted descending"
// @Success 204 "No trending articles in window (empty result, not an error)"
// @Failure 400 {object} map[string]string "Invalid query parameters"
// @Failure 401 {object} map[string]string "Missing or invalid API key"
// @Failure 429 {object} map[string]string "Concurrency limit exceeded; retry shortly"
// @Failure 500 {object} map[string]string "Database or embedder unavailable; retry"
// @ID getTrendingArticles
// @Router /articles/trending [get]
func (r *Configuration) getTrendingArticles(c *gin.Context) {
	conditions := c.MustGet("req_conditions").(bs.Condition)
	page := c.MustGet("req_page").(bs.Pagination)
	columns := c.MustGet("req_columns").([]string)
	// default to last 7 days if trending window provided
	if conditions.Updated.IsZero() {
		conditions.Updated = time.Now().AddDate(0, 0, -DEFAULT_WINDOW)
	}
	conditions.Created = time.Time{}
	items, err := r.DB.QueryTrendingBeans(c.Request.Context(), conditions, page, append(columns, _BEAN_TREND_FIELDS))
	returnResponse(c, items, err)
}

// getTopHeadlinesArticles godoc
// @Summary Search or list top headlines from the last 24 hours
// @Description Returns the highest trend_score articles from the past 24 hours — a narrow window on getTrendingArticles.
// @Description **When to use**: breaking news, daily briefings, or "what is hot today" without a custom date range.
// @Description **Note**: `from` is not accepted; the 24h window is fixed server-side.
// @Description **Filters** (all optional): same semantics as getTrendingArticles except no date override.
// @Description **Related tools**: getTrendingArticles (7-day window), searchArticles.
// @Tags Articles
// @Accept json
// @Produce json
// @Param q query string false "Optional semantic search query (3–512 chars)."
// @Param acc query number false "Match strictness when q is set. Default 0.75." default(0.75) minimum(0) maximum(1)
// @Param content_type query string false "Restrict to content kind: news or blog." Enums(news,blog)
// @Param tags query []string false "Fuzzy filter across categories+regions+entities (AND between values)." collectionFormat(csv)
// @Param categories query []string false "Exact topic filter (OR)." collectionFormat(csv)
// @Param regions query []string false "Exact region filter (OR)." collectionFormat(csv)
// @Param entities query []string false "Exact entity filter (OR)." collectionFormat(csv)
// @Param sources query []string false "Publisher source ID filter (OR)." collectionFormat(csv)
// @Param full_content query bool false "Include full article body. Default false." default(false)
// @Param limit query int false "Page size. Default 16, max 128." default(16) minimum(1) maximum(128)
// @Param offset query int false "Skip N results. Default 0." minimum(0)
// @Success 200 {array} beansack.BeanTrend "Top headlines from last 24h sorted by trend_score descending"
// @Success 204 "No headlines in last 24h (empty result, not an error)"
// @Failure 400 {object} map[string]string "Invalid query parameters"
// @Failure 401 {object} map[string]string "Missing or invalid API key"
// @Failure 429 {object} map[string]string "Concurrency limit exceeded; retry shortly"
// @Failure 500 {object} map[string]string "Database or embedder unavailable; retry"
// @ID getTopHeadlines
// @Router /articles/top-headlines [get]
func (r *Configuration) getTopHeadlinesArticles(c *gin.Context) {
	conditions := c.MustGet("req_conditions").(bs.Condition)
	page := c.MustGet("req_page").(bs.Pagination)
	columns := c.MustGet("req_columns").([]string)
	conditions.Created = time.Now().AddDate(0, 0, -MIN_WINDOW) // last 24 hours
	conditions.Updated = time.Now().AddDate(0, 0, -MIN_WINDOW)
	items, err := r.DB.QueryTrendingBeans(c.Request.Context(), conditions, page, append(columns, _BEAN_TREND_FIELDS))
	returnResponse(c, items, err)
}

// getArticlePropagation godoc
// @Summary Track how articles spread (GET)
// @Description For each seed article URL, returns cross-outlet republication (`coverage`) and social/forum mentions (`mentions`).
// @Description **Input**: pass up to 128 article URLs as comma-separated query param `urls`.
// @Description **When to use**: after searchArticles — measure whether a story was picked up elsewhere or discussed on social platforms.
// @Description **Returns**: one PropagationResult per input URL (always HTTP 200; empty arrays when no propagation found).
// @Description **Related tools**: searchArticles, getTrendingArticles.
// @Tags Articles
// @Produce json
// @Param urls query []string true "Seed article URLs to analyze (CSV, 1–128 valid HTTP(S) URLs)" collectionFormat(csv)
// @Success 200 {array} beansack.PropagationResult "One result object per input URL with coverage and mentions arrays"
// @Failure 400 {object} map[string]string "Missing urls, too many urls (>128), or invalid URL format"
// @Failure 401 {object} map[string]string "Missing or invalid API key"
// @Failure 429 {object} map[string]string "Concurrency limit exceeded; retry shortly"
// @Failure 500 {object} map[string]string "Database unavailable; retry"
// @ID getArticlePropagation
// @Router /articles/propagation [get]
func (r *Configuration) getArticlePropagation(c *gin.Context) {
	r.handleArticlePropagation(c)
}

// postArticlePropagation godoc
// @Summary Track how articles spread (POST)
// @Description Same as getArticlePropagation but accepts a JSON body — preferred when URLs contain characters awkward in query strings.
// @Description **Input**: JSON body `{ "urls": ["https://...", ...] }` with 1–128 valid HTTP(S) URLs.
// @Description **When to use**: batch propagation lookup from agent workflows that already hold URL lists in JSON.
// @Tags Articles
// @Accept json
// @Produce json
// @Param input body PropagationInput true "JSON object with urls array (1–128 seed article URLs)"
// @Success 200 {array} beansack.PropagationResult "One result object per input URL with coverage and mentions arrays"
// @Failure 400 {object} map[string]string "Missing urls, too many urls (>128), or invalid URL format"
// @Failure 401 {object} map[string]string "Missing or invalid API key"
// @Failure 429 {object} map[string]string "Concurrency limit exceeded; retry shortly"
// @Failure 500 {object} map[string]string "Database unavailable; retry"
// @ID postArticlePropagation
// @Router /articles/propagation [post]
func (r *Configuration) postArticlePropagation(c *gin.Context) {
	r.handleArticlePropagation(c)
}

func (r *Configuration) handleArticlePropagation(c *gin.Context) {
	input, err := bindPropagationInput(c)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	items, err := r.DB.QueryPropagation(c.Request.Context(), input.URLs)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": _DB_ERROR})
		return
	}
	c.JSON(http.StatusOK, items)
}

func NewRouter(db bs.Beansack, embedder nlp.Embedder, api_keys map[string]string, max_concurrent_requests int) *gin.Engine {
	if max_concurrent_requests <= 0 {
		max_concurrent_requests = DEFAULT_CONCURRENCY // default to 100 if not set or invalid
	}
	config := &Configuration{
		DB:       db,
		Embedder: embedder,
		APIKeys:  api_keys,
		queue:    make(chan int, max_concurrent_requests),
		cache: otter.Must(&otter.Options[string, []float32]{
			MaximumSize:      _CACHE_SIZE,
			ExpiryCalculator: otter.ExpiryAccessing[string, []float32](_CACHE_TTL),
		}),
	}

	router := gin.New()
	// JSON access logs and recovery using zerolog
	router.Use(
		// logger
		requestLogger,
		// recovery
		gin.Recovery(),
		// cors
		cors.New(cors.Config{
			AllowAllOrigins:  true,
			AllowMethods:     []string{"GET", "POST", "OPTIONS"},
			AllowHeaders:     []string{"*"},
			AllowCredentials: false,
			MaxAge:           24 * time.Hour,
		}),
	)

	// Swagger / OpenAPI endpoints
	// NOTE: run `swag init` to generate docs (package `docs`) before using the UI.
	// Serve Swagger UI and point it at the generated spec in assets/docs
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	router.GET("/health", config.health)
	router.StaticFile("favicon.ico", FAVICON_PATH)

	// protected group
	protected := router.Group("/")
	protected.Use(config.apiKeyMiddleware, config.concurrencyMiddleware)

	tags := protected.Group("/tags", validateTagsParams)
	{
		tags.GET("/categories", config.getCategories)
		tags.GET("/entities", config.getEntities)
		tags.GET("/regions", config.getRegions)
	}
	publishers := protected.Group("/sources", validatePublishersParams)
	{
		publishers.GET("", config.getPublishers)
		// publishers.GET("/metadata", config.getPublishers)
	}
	articles := protected.Group("/articles", config.validateArticlesParams)
	{
		articles.GET("/search", config.searchArticles)
		articles.GET("/latest", config.getLatestArticles)
		articles.GET("/trending", config.getTrendingArticles)
		articles.GET("/top-headlines", config.getTopHeadlinesArticles)
	}
	protected.GET("/articles/propagation", config.getArticlePropagation)
	protected.POST("/articles/propagation", config.postArticlePropagation)
	return router
}

// Middleware
func (r *Configuration) apiKeyMiddleware(c *gin.Context) {
	if len(r.APIKeys) == 0 {
		c.Next()
		return
	}
	for header, expected := range r.APIKeys {
		if strings.TrimSpace(c.GetHeader(header)) == expected {
			c.Next()
			return
		}
	}
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing API Key"})
}

func (r *Configuration) concurrencyMiddleware(c *gin.Context) {
	if r.queue != nil {
		r.queue <- 1
		defer func() { <-r.queue }()
	}
	c.Next()
}

// requestLogger logs request path, query parameters, status and latency in JSON via zerolog
func requestLogger(c *gin.Context) {
	start := time.Now()
	c.Next()

	status := c.Writer.Status()

	var evt *zerolog.Event
	if len(c.Errors) > 0 || status >= 500 {
		evt = log.Error()
	} else if status >= 400 {
		evt = log.Warn()
	} else {
		evt = log.Info()
	}
	evt.Str("module", "ROUTER").Str("method", c.Request.Method).
		Str("path", c.Request.URL.Path).
		Interface("query", c.Request.URL.Query()).
		Int("status", status).
		Float64("latency", time.Since(start).Seconds())

	if len(c.Errors) > 0 {
		evt.Str("error", c.Errors.String())
	}
	evt.Msg("incoming")
}

func returnResponse[T any](c *gin.Context, items []T, err error) {
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": _DB_ERROR})
		return
	}
	if len(items) == 0 {
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, items)

}
