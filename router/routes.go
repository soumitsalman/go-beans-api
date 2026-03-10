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
	"fmt"
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
	DEFAULT_ACCURACY    = 0.75
	DEFAULT_LIMIT       = 16
	MAX_LIMIT           = 128
	FAVICON_PATH        = "./images/beans.png"
	DEFAULT_CONCURRENCY = 512
)

const (
	_EMBEDDER_ERROR = "Embedder just died. Retry in a bit."
	_DB_ERROR       = "DB just died. Retry in a bit."
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

// TagsInput describes parameters for tag queries
// Description: Query for tag-like resources (categories, entities, regions).
type PublishersInput struct {
	Sources []string `form:"sources"`
	PaginationInput
}

// PublishersInput describes parameters for publisher queries
// Description: Query parameters used to filter publishers by source(s).
type ArticlesInput struct {
	Q              string    `form:"q" binding:"max=512"`
	Acc            float64   `form:"acc,default=0.75" binding:"min=0,max=1"`
	Kind           string    `form:"kind"`
	Tags           []string  `form:"tags" collection_format:"multi"`
	Sources        []string  `form:"sources" collection_format:"multi"`
	PublishedSince time.Time `form:"published_since" time_format:"2006-01-02" swaggertype:"string" format:"date"`
	TrendingSince  time.Time `form:"trending_since" time_format:"2006-01-02" swaggertype:"string" format:"date"`
	WithContent    bool      `form:"with_content,default=false"`
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
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.Set("req_params", input)
	c.Set("req_page", bs.Pagination{Limit: input.Limit, Offset: input.Offset})
	c.Next()
}

// getCategories godoc
// @Summary List categories
// @Description Retrieves a list of unique values of article categories/topics (paginated), for example: Artificial Intelligence, Cybersecurity, Politics, Software Engineering, etc.
// @Tags Tags
// @Produce json
// @Param limit query int false "limit" default(16) minimum(1) maximum(128)
// @Param offset query int false "offset"
// @Success 200 {array} string
// @Failure 500 {string} string "error message"
// @Router /tags/categories [get]
func (r *Configuration) getCategories(c *gin.Context) {
	page := c.MustGet("req_page").(bs.Pagination)
	data, err := r.DB.DistinctCategories(c.Request.Context(), page)
	returnResponse(c, data, err)
}

// getEntities godoc
// @Summary List entities
// @Description Retrieves a list of unique named entities (paginated), such as people, organizations, and products mentioned in articles.
// @Tags Tags
// @Produce json
// @Param limit query int false "limit" default(16) minimum(1) maximum(128)
// @Param offset query int false "offset"
// @Success 200 {array} string
// @Failure 500 {string} string "error message"
// @Router /tags/entities [get]
func (r *Configuration) getEntities(c *gin.Context) {
	page := c.MustGet("req_page").(bs.Pagination)
	data, err := r.DB.DistinctEntities(c.Request.Context(), page)
	returnResponse(c, data, err)
}

// getRegions godoc
// @Summary List regions
// @Description Retrieves a list of unique geographic regions (paginated) mentioned in articles, e.g., UK, US, Europe.
// @Tags Tags
// @Produce json
// @Param limit query int false "limit" default(16) minimum(1) maximum(128)
// @Param offset query int false "offset"
// @Success 200 {array} string
// @Failure 500 {string} string "error message"
// @Router /tags/regions [get]
func (r *Configuration) getRegions(c *gin.Context) {
	page := c.MustGet("req_page").(bs.Pagination)
	data, err := r.DB.DistinctRegions(c.Request.Context(), page)
	returnResponse(c, data, err)
}

func validatePublishersParams(c *gin.Context) {
	var input PublishersInput
	if err := c.ShouldBindQuery(&input); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	c.Set("req_params", input)
	c.Set("req_conditions", bs.Condition{Sources: input.Sources})
	c.Set("req_page", bs.Pagination{Limit: input.Limit, Offset: input.Offset})
	c.Next()
}

// getSources godoc
// @Summary List sources
// @Description Retrieves a list of unique publisher IDs (paginated) from which articles are sourced.
// @Tags Publishers
// @Produce json
// @Param limit query int false "limit" default(16) minimum(1) maximum(128)
// @Param offset query int false "offset"
// @Success 200 {array} string
// @Failure 500 {string} string "error message"
// @Router /publishers/sources [get]
func (r *Configuration) getSources(c *gin.Context) {
	page := c.MustGet("req_page").(bs.Pagination)
	items, err := r.DB.DistinctSources(c.Request.Context(), page)
	returnResponse(c, items, err)
}

// getPublishers godoc
// @Summary Query publishers
// @Description Retrieves publisher metadata filtered by one or more publisher IDs.
// @Tags Publishers
// @Produce json
// @Param sources query []string true "sources/publisher ids to include" collectionFormat(multi)
// @Param limit query int false "limit" default(16) minimum(1) maximum(128)
// @Param offset query int false "offset"
// @Success 200 {array} beansack.Publisher
// @Failure 400 {string} string "error message"
// @Failure 500 {string} string "error message"
// @Router /publishers [get]
func (r *Configuration) getPublishers(c *gin.Context) {
	conditions := c.MustGet("req_conditions").(bs.Condition)
	page := c.MustGet("req_page").(bs.Pagination)
	if len(conditions.Sources) == 0 {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("source parameter required"))
		return
	}
	items, err := r.DB.QueryPublishers(c.Request.Context(), conditions, page, []string{bs.CORE_PUBLISHER_FIELDS})
	returnResponse(c, items, err)
}

func (config *Configuration) validateArticlesParams(c *gin.Context) {
	var input ArticlesInput
	if err := c.ShouldBindQuery(&input); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	conditions := bs.Condition{
		Kind:    input.Kind,
		Created: input.PublishedSince,
		Updated: input.TrendingSince,
		Tags:    input.Tags,
		Sources: input.Sources,
		Extra:   []string{bs.PROCESSED_BEANS_CONDITIONS},
	}
	if input.WithContent {
		conditions.Extra = append(conditions.Extra, bs.UNRESTRICTED_CONTENT_CONDITIONS)
	}
	if input.Q != "" {
		conditions.Distance = 1 - input.Acc
		conditions.Embedding = config.Embedder.EmbedQuery(c, input.Q)
		if len(conditions.Embedding) == 0 {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf(_EMBEDDER_ERROR))
			return
		}
	}
	columns := []string{bs.CORE_BEAN_FIELDS}
	if input.WithContent {
		columns = []string{bs.K_CONTENT}
	}
	c.Set("req_params", input)
	c.Set("req_conditions", conditions)
	c.Set("req_page", bs.Pagination{Limit: input.Limit, Offset: input.Offset})
	c.Set("req_columns", columns)
	c.Next()
}

// getLatestArticles godoc
// @Summary Get latest articles
// @Description Searches for the latest articles/news/blogs. For vector search (when `q` is provided), results are sorted by relevance; otherwise results are sorted by publication date (newest first).
// @Tags Articles
// @Accept json
// @Produce json
// @Param q query string false "search query (min length 3, max length 512)" maxlength(512)
// @Param acc query number false "accuracy (0-1) used as cosine similarity threshold; higher values return fewer, more similar results" default(0.75) minimum(0) maximum(1)
// @Param kind query string false "kind filter (news, blog, etc.)"
// @Param tags query []string false "tags (categories, regions, entities) to filter by" collectionFormat(multi)
// @Param sources query []string false "sources/publisher ids to filter by" collectionFormat(multi)
// @Param published_since query string false "published since (YYYY-MM-DD)" format(date) default(last 7 days)
// @Param with_content query bool false "include content" default(false)
// @Param limit query int false "limit" default(16) minimum(1) maximum(128)
// @Param offset query int false "offset"
// @Success 200 {array} beansack.Bean
// @Failure 400 {string} string "error message"
// @Failure 500 {string} string "error message"
// @Router /articles/latest [get]
func (r *Configuration) getLatestArticles(c *gin.Context) {
	conditions := c.MustGet("req_conditions").(bs.Condition)
	page := c.MustGet("req_page").(bs.Pagination)
	columns := c.MustGet("req_columns").([]string)
	if conditions.Created.IsZero() {
		conditions.Created = time.Now().AddDate(0, 0, -7) // default to last 7 days if no published filter provided
	}
	items, err := r.DB.QueryLatestBeans(c.Request.Context(), conditions, page, columns)
	returnResponse(c, items, err)
}

// getTrendingArticles godoc
// @Summary Get trending articles
// @Description Searches for trending articles/news/blogs. For vector search (when `q` is provided), results are sorted by relevance; otherwise they are sorted by an internal trend score computed from social engagement metrics (comments, likes, shares, last engagement, etc.).
// @Tags Articles
// @Accept json
// @Produce json
// @Param q query string false "search query (min length 3, max length 512)" maxlength(512)
// @Param acc query number false "accuracy (0-1) used as cosine similarity threshold; higher values return fewer, more similar results" default(0.75) minimum(0) maximum(1)
// @Param kind query string false "kind filter (news, blog, etc.)"
// @Param tags query []string false "tags (categories, regions, entities) to filter by" collectionFormat(multi)
// @Param sources query []string false "sources/publisher ids to filter by" collectionFormat(multi)
// @Param trending_since query string false "trending since (YYYY-MM-DD)" format(date) default(last 7 days)
// @Param with_content query bool false "include content" default(false)
// @Param limit query int false "limit" default(16) minimum(1) maximum(128)
// @Param offset query int false "offset"
// @Success 200 {array} beansack.Bean
// @Failure 400 {string} string "error message"
// @Failure 500 {string} string "error message"
// @Router /articles/trending [get]
func (r *Configuration) getTrendingArticles(c *gin.Context) {
	conditions := c.MustGet("req_conditions").(bs.Condition)
	page := c.MustGet("req_page").(bs.Pagination)
	columns := c.MustGet("req_columns").([]string)
	if conditions.Updated.IsZero() {
		conditions.Updated = time.Now().AddDate(0, 0, -7) // default to last 7 days if no trending filter provided
	}
	items, err := r.DB.QueryTrendingBeans(c.Request.Context(), conditions, page, append(columns, bs.K_TRENDSCORE))
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
	publishers := protected.Group("/publishers", validatePublishersParams)
	{
		publishers.GET("", config.getPublishers)
		publishers.GET("/sources", config.getSources)
	}
	articles := protected.Group("/articles", config.validateArticlesParams)
	{
		articles.GET("/latest", config.getLatestArticles)
		articles.GET("/trending", config.getTrendingArticles)
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
	c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("Missing API Key"))
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
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf(_DB_ERROR))
		return
	}
	if len(items) == 0 {
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, items)
}
