// @title 			Beans API & MCP
// @version 		0.1
// @description 	Beans is an intelligent news & blogs aggregation and search service that curates fresh content from RSS feeds using AI-powered natural language queries and filters.
// @schemes 		https
// @license.name 	MIT
// @contact.name 	Project Cafecito
// @contact.url  	http://cafecito.tech
// @contact.email 	soumitsrah@cafecito.tech
package router

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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
	_EMBEDDER_ERROR     = "Embedder just died. Retry in a bit."
	_DB_ERROR           = "DB just died. Retry in a bit."
	_NEEDS_SEARCH_PARAM = "At least one search parameter is required (q, tags, categories, regions, entities)."
)

const (
	_BEAN_TREND_FIELDS = "likes, comments, shares, trend_score"
)

type PaginationInput struct {
	Limit  int `form:"limit,default=16" binding:"min=1,max=128"`
	Offset int `form:"offset" binding:"min=0"`
}

// PaginationInput describes common pagination query params
// Description: Common pagination parameters used by list endpoints.
type TagsInput struct {
	PaginationInput
}

// Description: Query for tag-like resources (categories, entities, regions).
type PublishersInput struct {
	// Sources: Publisher/source IDs to include (CSV).
	Sources []string `form:"sources" collection_format:"csv"`
	PaginationInput
}

// PublishersInput describes parameters for publisher queries
// Description: Query parameters used to filter publishers by source(s).
// ArticlesInput contains query parameters for article list/search endpoints.
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

type Configuration struct {
	DB       bs.Beansack
	Embedder nlp.Embedder
	APIKeys  map[string]string
	queue    chan int
}

// health
func (r *Configuration) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "alive"})
}

// health godoc
// @Summary Health check
// @Description Returns service health status
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health [get]
func validateTagsParams(c *gin.Context) {
	var input TagsInput
	if err := c.ShouldBindQuery(&input); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Set("req_params", input)
	c.Set("req_page", bs.Pagination{Limit: input.Limit, Offset: input.Offset})
	c.Next()
}

// getCategories godoc
// @Summary List article categories
// @Description Retrieves a paginated list of unique article categories/topics discovered in the database.
// Examples: Artificial Intelligence, Cybersecurity, Politics, Software Engineering, Business, Healthcare, Technology, etc.
// @Tags Tags
// @Produce json
// @Param limit query int false "page limit (items per page)" default(16) minimum(1) maximum(128)
// @Param offset query int false "pagination offset (number of items to skip)"
// @Success 200 {array} string "list of category strings"
// @Failure 401 {object} map[string]string "unauthorized: missing or invalid API key"
// @Failure 500 {object} map[string]string "database error"
// @Router /tags/categories [get]
func (r *Configuration) getCategories(c *gin.Context) {
	page := c.MustGet("req_page").(bs.Pagination)
	data, err := r.DB.DistinctCategories(c.Request.Context(), page)
	returnResponse(c, data, err)
}

// getEntities godoc
// @Summary List named entities
// @Description Retrieves a paginated list of unique named entities (persons, organizations, products, places) extracted from articles.
// Entities are discovered using NLP and represent key concepts mentioned across content.
// @Tags Tags
// @Produce json
// @Param limit query int false "page limit (items per page)" default(16) minimum(1) maximum(128)
// @Param offset query int false "pagination offset (number of items to skip)"
// @Success 200 {array} string "list of entity strings"
// @Failure 401 {object} map[string]string "unauthorized: missing or invalid API key"
// @Failure 500 {object} map[string]string "database error"
// @Router /tags/entities [get]
func (r *Configuration) getEntities(c *gin.Context) {
	page := c.MustGet("req_page").(bs.Pagination)
	data, err := r.DB.DistinctEntities(c.Request.Context(), page)
	returnResponse(c, data, err)
}

// getRegions godoc
// @Summary List geographic regions
// @Description Retrieves a paginated list of unique geographic regions mentioned in articles.
// Examples: North America, Europe, Asia, UK, US, France, India, Australia, Middle East, etc.
// @Tags Tags
// @Produce json
// @Param limit query int false "page limit (items per page)" default(16) minimum(1) maximum(128)
// @Param offset query int false "pagination offset (number of items to skip)"
// @Success 200 {array} string "list of region strings"
// @Failure 401 {object} map[string]string "unauthorized: missing or invalid API key"
// @Failure 500 {object} map[string]string "database error"
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
	c.Set("req_params", input)
	c.Set("req_conditions", bs.Condition{Sources: input.Sources})
	c.Set("req_page", bs.Pagination{Limit: input.Limit, Offset: input.Offset})
	c.Next()
}

// getSources godoc
// @Summary List available sources
// @Description Retrieves a paginated list of unique publisher source IDs and identifiers.
// Use these source IDs with other endpoints to filter content by specific publishers.
// @Tags Publishers
// @Produce json
// @Param limit query int false "page limit (items per page)" default(16) minimum(1) maximum(128)
// @Param offset query int false "pagination offset (number of items to skip)"
// @Success 200 {array} string "list of source/publisher ID strings"
// @Failure 401 {object} map[string]string "unauthorized: missing or invalid API key"
// @Failure 500 {object} map[string]string "database error"
// @Router /publishers/sources [get]
func (r *Configuration) getSources(c *gin.Context) {
	page := c.MustGet("req_page").(bs.Pagination)
	items, err := r.DB.DistinctSources(c.Request.Context(), page)
	returnResponse(c, items, err)
}

// getPublishers godoc
// @Summary Query source metadata
// @Description Retrieves detailed metadata for one or more sources including site name, description, favicon, and RSS feed info.
// Requires at least one source ID. Use /sources endpoint to discover available source IDs.
// @Tags Publishers
// @Produce json
// @Param sources query []string true "source IDs to fetch metadata for (required, comma-separated CSV)" collectionFormat(csv)
// @Param limit query int false "page limit (items per page)" default(16) minimum(1) maximum(128)
// @Param offset query int false "pagination offset (number of items to skip)"
// @Success 200 {array} beansack.Publisher "array of publisher metadata objects"
// @Failure 400 {object} map[string]string "bad request: missing or invalid sources parameter"
// @Failure 401 {object} map[string]string "unauthorized: missing or invalid API key"
// @Failure 500 {object} map[string]string "database error"
// @Router /publishers [get]
func (r *Configuration) getPublishers(c *gin.Context) {
	conditions := c.MustGet("req_conditions").(bs.Condition)
	page := c.MustGet("req_page").(bs.Pagination)
	if len(conditions.Sources) == 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Missing required parameter: sources"})
		return
	}
	items, err := r.DB.QueryPublishers(c.Request.Context(), conditions, page, []string{bs.CORE_PUBLISHER_FIELDS})
	returnResponse(c, items, err)
}

func (config *Configuration) validateArticlesParams(c *gin.Context) {
	var input ArticlesInput
	if err := c.ShouldBindQuery(&input); err != nil {
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
		Extra:      []string{bs.PROCESSED_BEANS_CONDITIONS},
	}
	if input.FullContent {
		conditions.Extra = append(conditions.Extra, bs.UNRESTRICTED_CONTENT_CONDITIONS)
	}
	if input.Q != "" {
		conditions.Distance = 1 - input.Acc
		conditions.Embedding = config.Embedder.EmbedQuery(c, input.Q)
		if len(conditions.Embedding) == 0 {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": _EMBEDDER_ERROR})
			return
		}
	}
	columns := []string{bs.CORE_BEAN_FIELDS}
	if input.FullContent {
		columns = append(columns, bs.K_CONTENT)
	}
	c.Set("req_params", input)
	c.Set("req_conditions", conditions)
	c.Set("req_page", bs.Pagination{Limit: input.Limit, Offset: input.Offset})
	c.Set("req_columns", columns)
	c.Next()
}

// searchArticles godoc
// @Summary Semantic/tag-based article search
// @Description Perform semantic (vector embedding) or tag-based search across all articles in the database.
// Results return full article details with publisher info, engagement metrics, and computed trends.
// At least ONE of: `q`, `tags`, `categories`, `regions`, `entities`, or `urls` is required.
// Note: This is a heavy query; results sorted by relevance. Full content significantly increases payload size.
// For filtering: `tags` provides case/whitespace-insensitive text search across categories, regions, and entities (recommended starting point).
// For precision filtering, use `categories`, `regions`, `entities` (case/whitespace-sensitive). Get exhaustive tag lists from /tags/* endpoints.
// @Tags Articles
// @Accept json
// @Produce json
// @Param q query string false "semantic vector search query (character length 3-512, natural language)"
// @Param acc query number false "embedding accuracy/similarity threshold (0.0-1.0, higher = stricter match)" default(0.75) minimum(0) maximum(1)
// @Param content_type query string false "content type filter (news, blog, post, generated, comment, etc.)"
// @Param urls query []string false "specific article URLs to fetch directly (CSV)" collectionFormat(csv)
// @Param tags query []string false "case/whitespace-insensitive text search across categories, regions, entities (AND combination, recommended)" collectionFormat(csv)
// @Param categories query []string false "precise category topic filters (inclusive OR, case/whitespace-sensitive)" collectionFormat(csv)
// @Param regions query []string false "precise geographic region filters (inclusive OR, case/whitespace-sensitive)" collectionFormat(csv)
// @Param entities query []string false "precise named entity filters (inclusive OR, case/whitespace-sensitive)" collectionFormat(csv)
// @Param sources query []string false "publisher/source ID filters (inclusive OR)" collectionFormat(csv)
// @Param from query string false "published/updated since date (ISO 8601 date format YYYY-MM-DD)" format(date)
// @Param full_content query bool false "if true, include full article content (large payload)" default(false)
// @Param limit query int false "page limit (items per page)" default(16) minimum(1) maximum(128)
// @Param offset query int false "pagination offset (number of items to skip)"
// @Success 200 {array} beansack.BeanAggregate "array of article aggregates with engagement metrics"
// @Failure 400 {object} map[string]string "bad request: missing required search parameters or invalid input"
// @Failure 401 {object} map[string]string "unauthorized: missing or invalid API key"
// @Failure 500 {object} map[string]string "database or embedder error"
// @Router /articles/search [get]
// this one searches through the entire database and returns result sorted by relevance.
// this is a heavy query, it will be slow and higher network bandwidth
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
// @Summary Get latest articles (reverse chronological)
// @Description Retrieves the most recently published articles, sorted by publish date (newest first).
// Optionally filter by semantic search, categories, regions, entities, or publishers.
// If no `from` date provided, defaults to last 7 days. Results include publisher info and engagement metrics.
// For filtering: `tags` provides case/whitespace-insensitive text search across categories, regions, and entities (recommended starting point).
// For precision filtering, use `categories`, `regions`, `entities` (case/whitespace-sensitive). Get exhaustive tag lists from /tags/* endpoints.
// @Tags Articles
// @Accept json
// @Produce json
// @Param q query string false "optional semantic search query (character length 3-512)"
// @Param acc query number false "embedding accuracy/similarity threshold (0.0-1.0)" default(0.75) minimum(0) maximum(1)
// @Param kind query string false "content type filter (news, blog, post, etc.)"
// @Param tags query []string false "case/whitespace-insensitive text search across categories, regions, entities (recommended)" collectionFormat(csv)
// @Param categories query []string false "precise category filters (inclusive OR, case/whitespace-sensitive)" collectionFormat(csv)
// @Param regions query []string false "precise region filters (inclusive OR, case/whitespace-sensitive)" collectionFormat(csv)
// @Param entities query []string false "precise entity filters (inclusive OR, case/whitespace-sensitive)" collectionFormat(csv)
// @Param sources query []string false "publisher source filters (inclusive OR)" collectionFormat(csv)
// @Param from query string false "published since date (YYYY-MM-DD, defaults to 7 days ago if omitted)" format(date)
// @Param full_content query bool false "include full article content" default(false)
// @Param limit query int false "page limit" default(16) minimum(1) maximum(128)
// @Param offset query int false "pagination offset"
// @Success 200 {array} beansack.BeanAggregate "array of latest articles sorted by publish date"
// @Failure 400 {object} map[string]string "bad request: invalid parameters"
// @Failure 401 {object} map[string]string "unauthorized: missing or invalid API key"
// @Failure 500 {object} map[string]string "database or embedder error"
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
// @Summary Get trending articles
// @Description Retrieves trending articles ranked by trend score. Trend score is computed from:
// - Social engagement metrics (likes, comments, shares, subscriber reactions)
// - Publication coverage (number of sources publishing the same content)
// - Recency of engagement
// If no `from` date provided, defaults to last 7 days. Results sorted by trend score (highest first).
// Optionally filter by semantic search or other criteria.
// For filtering: `tags` provides case/whitespace-insensitive text search across categories, regions, and entities (recommended starting point).
// For precision filtering, use `categories`, `regions`, `entities` (case/whitespace-sensitive). Get exhaustive tag lists from /tags/* endpoints.
// @Tags Articles
// @Accept json
// @Produce json
// @Param q query string false "optional semantic search query (character length 3-512)"
// @Param acc query number false "embedding accuracy/similarity threshold (0.0-1.0, higher = stricter)" default(0.75) minimum(0) maximum(1)
// @Param kind query string false "content type filter (news, blog, post, etc.)"
// @Param tags query []string false "case/whitespace-insensitive text search across categories, regions, entities (recommended)" collectionFormat(csv)
// @Param categories query []string false "precise category filters (inclusive OR, case/whitespace-sensitive)" collectionFormat(csv)
// @Param regions query []string false "precise region filters (inclusive OR, case/whitespace-sensitive)" collectionFormat(csv)
// @Param entities query []string false "precise entity filters (inclusive OR, case/whitespace-sensitive)" collectionFormat(csv)
// @Param sources query []string false "publisher source filters (inclusive OR)" collectionFormat(csv)
// @Param from query string false "trending since date (YYYY-MM-DD, defaults to 7 days ago)" format(date)
// @Param full_content query bool false "include full article content" default(false)
// @Param limit query int false "page limit" default(16) minimum(1) maximum(128)
// @Param offset query int false "pagination offset"
// @Success 200 {array} beansack.BeanAggregate "array of trending articles sorted by trend_score (descending)"
// @Failure 400 {object} map[string]string "bad request: invalid parameters"
// @Failure 401 {object} map[string]string "unauthorized: missing or invalid API key"
// @Failure 500 {object} map[string]string "database or embedder error"
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
// @Summary Get top headlines (last 24 hours)
// @Description Retrieves top trending headlines from the past 24 hours, ranked by trend score.
// This is a specialized version of /articles/trending that uses a narrower time window for results.
// Useful for curating breaking news and most-discussed topics of the day.
// Optional filters apply the same as trending articles.
// For filtering: `tags` provides case/whitespace-insensitive text search across categories, regions, and entities (recommended starting point).
// For precision filtering, use `categories`, `regions`, `entities` (case/whitespace-sensitive). Get exhaustive tag lists from /tags/* endpoints.
// @Tags Articles
// @Accept json
// @Produce json
// @Param q query string false "optional semantic search query (character length 3-512)"
// @Param acc query number false "embedding accuracy/similarity threshold (0.0-1.0)" default(0.75) minimum(0) maximum(1)
// @Param kind query string false "content type filter (news, blog, post, etc.)"
// @Param tags query []string false "case/whitespace-insensitive text search across categories, regions, entities (recommended)" collectionFormat(csv)
// @Param categories query []string false "precise category filters (inclusive OR, case/whitespace-sensitive)" collectionFormat(csv)
// @Param regions query []string false "precise region filters (inclusive OR, case/whitespace-sensitive)" collectionFormat(csv)
// @Param entities query []string false "precise entity filters (inclusive OR, case/whitespace-sensitive)" collectionFormat(csv)
// @Param sources query []string false "publisher source filters (inclusive OR)" collectionFormat(csv)
// @Param full_content query bool false "include full article content" default(false)
// @Param limit query int false "page limit" default(16) minimum(1) maximum(128)
// @Param offset query int false "pagination offset"
// @Success 200 {array} beansack.BeanAggregate "array of top headlines from last 24h, sorted by trend_score"
// @Failure 400 {object} map[string]string "bad request: invalid parameters"
// @Failure 401 {object} map[string]string "unauthorized: missing or invalid API key"
// @Failure 500 {object} map[string]string "database or embedder error"
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

func NewRouter(db bs.Beansack, embedder nlp.Embedder, api_keys map[string]string, max_concurrent_requests int) *gin.Engine {
	if max_concurrent_requests <= 0 {
		max_concurrent_requests = DEFAULT_CONCURRENCY // default to 100 if not set or invalid
	}
	config := &Configuration{
		DB:       db,
		Embedder: embedder,
		APIKeys:  api_keys,
		queue:    make(chan int, max_concurrent_requests),
	}

	router := gin.New()
	// JSON access logs and recovery using zerolog
	router.Use(requestLogger, gin.Recovery())

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
		publishers.GET("", config.getSources)
		publishers.GET("/metadata", config.getPublishers)
	}
	articles := protected.Group("/articles", config.validateArticlesParams)
	{
		articles.GET("/search", config.searchArticles)
		articles.GET("/latest", config.getLatestArticles)
		articles.GET("/trending", config.getTrendingArticles)
		articles.GET("/top-headlines", config.getTopHeadlinesArticles)
	}
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
		Dur("latency", time.Since(start))

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
