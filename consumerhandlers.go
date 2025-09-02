package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/k0kubun/pp"
)

const (
	DEFAULT_LIMIT int64 = 50
	MIN_LIMIT     int64 = 1
	MAX_LIMIT     int64 = 300
)

var SELECT_PUBLIC_FIELDS = []string{"url", "kind", "title", "summary", "author", "source", "created", "categories", "sentiments", "regions", "entities", "updated", "likes", "comments", "shares"}
var SELECT_GISTS = []string{"url", "created", "updated", "gist", "categories", "sentiments", "regions", "entities"}
var SELECT_EMBEDDINGS = []string{"url", "created", "updated", "embedding"}
var ORDER_BY_CREATED = []string{"created DESC"}
var ORDER_BY_DISTANCE = []string{"distance ASC"}
var ORDER_BY_CHATTERS = []string{"DATE(updated) DESC", "comments DESC", "likes DESC", "shares DESC"}

type BeansQueryRequest struct {
	// these are query params
	Kind       string    `form:"kind"`
	Since      time.Time `form:"created_since" time_format:"2006-01-02T15:04:05Z07:00"`
	Categories []string  `form:"categories"`
	Regions    []string  `form:"regions"`
	Entities   []string  `form:"entities"`
	Domains    []string  `form:"domains"`
	Offset     int64     `form:"offset" json:"offset" binding:"min=0"`
	Limit      int64     `form:"limit" json:"limit" binding:"min=0,max=300" default:"50"`
	// these are body params
	Embedding   Float32Array `json:"embedding,omitempty"`
	MaxDistance float64      `json:"max_distance,omitempty" binding:"min=0,max=1"`
	URLs        []string     `json:"urls,omitempty"`
}

func validateBeansQueryRequest(c *gin.Context) {
	var req BeansQueryRequest
	err := c.ShouldBindQuery(&req)
	pp.Println("err", err)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err = c.ShouldBindJSON(&req)
	pp.Println("err", err)
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
	c.Set("req", req)
	c.Next()
}

func createQueryLatestBeansHandler(ds *Ducksack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(BeansQueryRequest)
		order_by := ORDER_BY_CREATED
		if len(req.Embedding) > 0 {
			order_by = ORDER_BY_DISTANCE
		}
		beans, err := findBeans(ds, req, nil, order_by, SELECT_PUBLIC_FIELDS)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, beans)
	}
}

func createRelatedBeansHandler(ds *Ducksack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(BeansQueryRequest)
		if len(req.URLs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "urls is required"})
			return
		}
		c.JSON(http.StatusOK, ds.GetRelated(req.URLs))
	}
}

func createRegionsHandler(ds *Ducksack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(BeansQueryRequest)
		if len(req.URLs) > 0 {
			c.JSON(http.StatusOK, ds.GetRegions(req.URLs))
		} else {
			c.JSON(http.StatusOK, ds.DistinctRegions())
		}
	}
}

func createEntitiesHandler(ds *Ducksack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(BeansQueryRequest)
		if len(req.URLs) > 0 {
			c.JSON(http.StatusOK, ds.GetEntities(req.URLs))
		} else {
			c.JSON(http.StatusOK, ds.DistinctEntities())
		}
	}
}

func createCategoriesHandler(ds *Ducksack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(BeansQueryRequest)
		if len(req.URLs) > 0 {
			c.JSON(http.StatusOK, ds.GetCategories(req.URLs))
		} else {
			c.JSON(http.StatusOK, ds.DistinctCategories())
		}
	}
}

func createSourcesHandler(ds *Ducksack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(BeansQueryRequest)
		if len(req.Domains) > 0 {
			c.JSON(http.StatusOK, ds.GetSources(req.Domains))
		} else {
			c.JSON(http.StatusOK, ds.DistinctSources())
		}
	}
}

////////// SORT BY TRENDING AND GET ALL FIELDS //////////

func validateVectorSearchRequest(c *gin.Context) {
	req := c.MustGet("req").(BeansQueryRequest)
	if len(req.Embedding) > 0 && req.MaxDistance == 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "For vector search, you must provide a max_distance"})
		return
	}
	c.Next()
}

func createTrendingBeansHandler(ds *Ducksack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(BeansQueryRequest)
		beans, err := findBeans(ds, req, nil, ORDER_BY_CHATTERS, nil)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, beans)
	}
}

func createTrendingBeanDigestsHandler(ds *Ducksack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(BeansQueryRequest)
		// pp.Println("req", req)
		beans, err := findBeans(ds, req, nil, ORDER_BY_CHATTERS, SELECT_GISTS)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		pp.Println("beans", len(beans))
		c.JSON(http.StatusOK, beans)
	}
}

func createTrendingBeanEmbeddingsHandler(ds *Ducksack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(BeansQueryRequest)
		beans, err := findBeans(ds, req, []string{"embedding IS NOT NULL"}, ORDER_BY_CHATTERS, SELECT_EMBEDDINGS)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, beans)
	}
}

func findBeans(ds *Ducksack, req BeansQueryRequest, additional_where []string, order_by []string, fields []string) ([]Bean, error) {
	if len(req.Embedding) > 0 {
		return ds.VectorSearchBeansWithSelectFields(
			req.Embedding,
			req.MaxDistance,
			req.Kind,
			req.Since,
			req.Categories,
			req.Regions,
			req.Entities,
			req.Domains,
			additional_where,
			order_by,
			req.Offset,
			req.Limit,
			fields,
		)
	} else {
		return ds.QueryBeansWithSelectFields(
			req.Kind,
			req.Since,
			req.Categories,
			req.Regions,
			req.Entities,
			req.Domains,
			additional_where,
			order_by,
			req.Offset,
			req.Limit,
			fields,
		)
	}
}
