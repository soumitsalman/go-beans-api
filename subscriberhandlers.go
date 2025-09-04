package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	DEFAULT_LIMIT int = 64
	MIN_LIMIT     int = 1
	MAX_LIMIT     int = 512
)

var SELECT_PUBLIC_FIELDS = []string{"url", "kind", "title", "summary", "author", "source", "created", "categories", "sentiments", "regions", "entities", "updated", "likes", "comments", "shares"}
var SELECT_GISTS = []string{"url", "created", "updated", "gist", "categories", "sentiments", "regions", "entities"}
var SELECT_EMBEDDINGS = []string{"url", "created", "updated", "embedding"}

type QueryRequest struct {
	// these custom fields. some of these can be passed as a query params
	Kind        string    `form:"kind" json:"kind"`
	Since       time.Time `form:"created_since" json:"created_since" time_format:"2006-01-02T15:04:05Z07:00"`
	Categories  []string  `form:"categories" json:"categories"`
	Regions     []string  `form:"regions" json:"regions"`
	Entities    []string  `form:"entities" json:"entities"`
	DomainNames []string  `form:"domains" json:"domain_names"`
	Embedding   []float32 `json:"embedding,omitempty"`
	MaxDistance float64   `json:"max_distance,omitempty" binding:"min=0,max=1"`
	URLs        []string  `json:"urls,omitempty"`

	// for more flexible query
	Where         []string `json:"where"`
	MissingFields []string `json:"missing_fields"`

	// for pagination
	Offset int `form:"offset" json:"offset" binding:"min=0"`
	Limit  int `form:"limit" json:"limit" binding:"min=0,max=512"`
}

func validateQueryRequest(c *gin.Context) {
	var req QueryRequest
	err := c.ShouldBindQuery(&req)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err = c.ShouldBindJSON(&req)
	// not having a body is not an error, malformed json is
	if err != nil && err.Error() != "EOF" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Embedding) > 0 && req.Limit == 0 && req.MaxDistance == 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "For vector search, you must provide either a limit or a max_distance"})
		return
	}
	// adjust the default
	if req.Limit == 0 {
		req.Limit = DEFAULT_LIMIT
	}
	c.Set("req", &req)
	c.Next()
}

func createLatestBeansHandler(ds *Ducksack, throttle chan int) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(*QueryRequest)
		order_by := ORDER_BY_CREATED
		if len(req.Embedding) > 0 {
			order_by = ORDER_BY_DISTANCE
		}
		query := buildQuery(ds, req).Columns(SELECT_PUBLIC_FIELDS...).Table(BEAN_AGGREGATES).Order(order_by)
		// beans := ds.QueryBeans(query)
		beans := runThrottledQuery(ds, query, req, throttle)
		c.JSON(http.StatusOK, beans)
	}
}

func createRelatedBeansHandler(ds *Ducksack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(*QueryRequest)
		if len(req.URLs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "urls is required"})
			return
		}
		c.JSON(http.StatusOK, ds.GetRelated(req.URLs))
	}
}

func createRegionsHandler(ds *Ducksack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(*QueryRequest)
		if len(req.URLs) > 0 {
			c.JSON(http.StatusOK, ds.GetRegions(req.URLs))
		} else {
			c.JSON(http.StatusOK, ds.DistinctRegions())
		}
	}
}

func createEntitiesHandler(ds *Ducksack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(*QueryRequest)
		if len(req.URLs) > 0 {
			c.JSON(http.StatusOK, ds.GetEntities(req.URLs))
		} else {
			c.JSON(http.StatusOK, ds.DistinctEntities())
		}
	}
}

func createCategoriesHandler(ds *Ducksack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(*QueryRequest)
		if len(req.URLs) > 0 {
			c.JSON(http.StatusOK, ds.GetCategories(req.URLs))
		} else {
			c.JSON(http.StatusOK, ds.DistinctCategories())
		}
	}
}

func createSourcesHandler(ds *Ducksack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(*QueryRequest)
		if len(req.DomainNames) > 0 {
			c.JSON(http.StatusOK, ds.GetSources(req.DomainNames))
		} else {
			c.JSON(http.StatusOK, ds.DistinctSources())
		}
	}
}

////////// PRIVILEGED //////////

func createExistsHandler(ds *Ducksack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(*QueryRequest)
		c.JSON(http.StatusOK, ds.Exists(req.URLs))
	}
}

func createLatestContentsHandler(ds *Ducksack, throttle chan int) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(*QueryRequest)
		query := buildQuery(ds, req).Table(BEAN_CORES).Order(ORDER_BY_CREATED)
		// beans := ds.QueryBeans(query)
		beans := runThrottledQuery(ds, query, req, throttle)
		c.JSON(http.StatusOK, beans)
	}
}

func createTrendingBeansHandler(ds *Ducksack, throttle chan int) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(*QueryRequest)
		query := buildQuery(ds, req).
			Table(BEAN_AGGREGATES).
			Where(HAS_CHATTERS).
			Order(ORDER_BY_UPDATED)
		// beans := ds.QueryBeans(query)
		beans := runThrottledQuery(ds, query, req, throttle)
		c.JSON(http.StatusOK, beans)
	}
}

func createTrendingTagsHandler(ds *Ducksack, throttle chan int) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(*QueryRequest)
		query := buildQuery(ds, req).
			Columns(SELECT_GISTS...).
			Table(BEAN_AGGREGATES).
			Where(GIST_IS_NOT_NULL, HAS_CHATTERS).
			Order(ORDER_BY_UPDATED)
		// beans := ds.QueryBeans(query)
		beans := runThrottledQuery(ds, query, req, throttle)
		c.JSON(http.StatusOK, beans)
	}
}

func createTrendingEmbeddingsHandler(ds *Ducksack, throttle chan int) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(*QueryRequest)
		query := buildQuery(ds, req).
			Columns(SELECT_EMBEDDINGS...).
			Table(BEAN_AGGREGATES).
			Where(HAS_CHATTERS).
			Order(ORDER_BY_UPDATED)
		// beans := ds.QueryBeans(query)
		beans := runThrottledQuery(ds, query, req, throttle)
		c.JSON(http.StatusOK, beans)
	}
}

func runThrottledQuery(ds *Ducksack, query *SelectExpr, req *QueryRequest, throttle chan int) []Bean {
	if req.Embedding != nil {
		throttle <- 1
	}
	beans := ds.QueryBeans(query)
	if req.Embedding != nil {
		<-throttle
	}
	return beans
}

func buildQuery(ds *Ducksack, req *QueryRequest) *SelectExpr {
	query := NewSelect(ds).
		Where(req.Where...).
		Offset(req.Offset).
		Limit(req.Limit)
	query = whereForCustomColumns(
		query,
		req.URLs,
		req.Kind,
		req.Since,
		req.Categories,
		req.Regions,
		req.Entities,
		req.DomainNames,
		req.Embedding,
		req.MaxDistance,
	)
	query = whereColumnNotExists(query, req.MissingFields...)
	return query
}

func whereForCustomColumns(q *SelectExpr,
	urls []string,
	kind string,
	created_after time.Time,
	categories []string,
	regions []string,
	entities []string,
	sources []string,
	embedding []float32,
	max_distance float64,
) *SelectExpr {
	if len(urls) > 0 {
		q.where = append(q.where, ExprWithArg{expr: "url IN (?)", arg: urls})
	}
	if kind != "" {
		q.where = append(q.where, ExprWithArg{expr: "kind = ?", arg: kind})
	}
	if !created_after.IsZero() {
		q.where = append(q.where, ExprWithArg{expr: "created >= ?", arg: created_after})
	}
	if len(categories) > 0 {
		q.where = append(q.where, ExprWithArg{expr: "ARRAY_HAS_ANY(categories, ?)", arg: StringArray(categories)})
	}
	if len(regions) > 0 {
		q.where = append(q.where, ExprWithArg{expr: "ARRAY_HAS_ANY(regions, ?)", arg: StringArray(regions)})
	}
	if len(entities) > 0 {
		q.where = append(q.where, ExprWithArg{expr: "ARRAY_HAS_ANY(entities, ?)", arg: StringArray(entities)})
	}
	if len(sources) > 0 {
		q.where = append(q.where, ExprWithArg{expr: "source IN (?)", arg: sources})
	}
	if embedding != nil {
		q.columns = append(q.columns, ExprWithArg{expr: fmt.Sprintf("array_cosine_distance(embedding, ?::FLOAT[%d]) AS distance", q.dim), arg: Float32Array(embedding)})
	}
	if max_distance > 0 {
		q.where = append(q.where, ExprWithArg{expr: "distance <= ?", arg: max_distance})
	}
	return q
}

const _SQL_MISSING_COLUMN = "url NOT IN (SELECT url FROM %s)"

func whereColumnNotExists(q *SelectExpr, columns ...string) *SelectExpr {
	for _, column := range columns {
		switch column {
		case "gist":
			q.Where(fmt.Sprintf(_SQL_MISSING_COLUMN, BEAN_GISTS))
		case "embedding":
			q.Where(fmt.Sprintf(_SQL_MISSING_COLUMN, BEAN_EMBEDDINGS))
		case "category":
			q.Where(fmt.Sprintf(_SQL_MISSING_COLUMN, BEAN_CATEGORIES))
		case "sentiment":
			q.Where(fmt.Sprintf(_SQL_MISSING_COLUMN, BEAN_SENTIMENTS))
		case "region":
		case "regions":
			q.Where(fmt.Sprintf(_SQL_MISSING_COLUMN, BEAN_REGIONS))
		case "entity":
		case "entities":
			q.Where(fmt.Sprintf(_SQL_MISSING_COLUMN, BEAN_ENTITIES))
		}
	}
	return q
}
