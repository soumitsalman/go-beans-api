package router

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	bs "github.com/soumitsalman/gobeansack/beansack"
)

const (
	DEFAULT_WINDOW       = 7 // days
	DEFAULT_LIMIT  int64 = 50
	MIN_LIMIT      int64 = 1
	MAX_LIMIT      int64 = 300
)

type BeansQueryRequest struct {
	URLs         []string        `json:"urls,omitempty"`
	Kind         string          `form:"kind"`
	Authors      []string        `form:"authors"`
	Sources      []string        `form:"sources"`
	CreatedSince time.Time       `form:"created_since" time_format:"2006-01-02T15:04:05Z07:00"`
	UpdatedSince time.Time       `form:"updated_since" time_format:"2006-01-02T15:04:05Z07:00"`
	Categories   []string        `form:"categories"`
	Regions      []string        `form:"regions"`
	Entities     []string        `form:"entities"`
	Embedding    bs.Float32Array `json:"embedding,omitempty"`
	MaxDistance  float64         `json:"max_distance,omitempty" binding:"min=0,max=1"`
	Offset       int64           `form:"offset" json:"offset" binding:"min=0"`
	Limit        int64           `form:"limit" json:"limit" binding:"min=0,max=300" default:"50"`
}

func validateBeansQueryRequest(c *gin.Context) {
	var req BeansQueryRequest
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
	if req.CreatedSince.IsZero() {
		req.CreatedSince = time.Now().AddDate(0, 0, -DEFAULT_WINDOW) // default to last 30 days
	}
	if req.Limit == 0 {
		req.Limit = DEFAULT_LIMIT
	}
	c.Set("req", req)
	c.Next()
}

func createRelatedBeansHandler(db *bs.Beansack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(BeansQueryRequest)
		if len(req.URLs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "urls is required"})
			return
		}
		c.JSON(http.StatusOK, db.GetRelated(req.URLs))
	}
}

func createRegionsHandler(db *bs.Beansack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(BeansQueryRequest)
		if len(req.URLs) > 0 {
			c.JSON(http.StatusOK, db.GetRegions(req.URLs))
		} else {
			c.JSON(http.StatusOK, db.DistinctRegions())
		}
	}
}

func createEntitiesHandler(db *bs.Beansack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(BeansQueryRequest)
		if len(req.URLs) > 0 {
			c.JSON(http.StatusOK, db.GetEntities(req.URLs))
		} else {
			c.JSON(http.StatusOK, db.DistinctEntities())
		}
	}
}

func createCategoriesHandler(db *bs.Beansack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(BeansQueryRequest)
		if len(req.URLs) > 0 {
			c.JSON(http.StatusOK, db.GetCategories(req.URLs))
		} else {
			c.JSON(http.StatusOK, db.DistinctCategories())
		}
	}
}

func createSourcesHandler(db *bs.Beansack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(BeansQueryRequest)
		if len(req.Sources) > 0 {
			c.JSON(http.StatusOK, db.GetSources(req.Sources))
		} else {
			c.JSON(http.StatusOK, db.DistinctSources())
		}
	}
}

func createLatestBeansHandler(db *bs.Beansack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(BeansQueryRequest)
		beans, err := findBeans(db, "latest", req, bs.PUBLIC_COLUMNS)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, beans)
	}
}

func createTrendingBeansHandler(db *bs.Beansack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(BeansQueryRequest)
		beans, err := findBeans(db, "trending", req, bs.PUBLIC_COLUMNS)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, beans)
	}
}

func createLatestDigestsHandler(db *bs.Beansack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(BeansQueryRequest)
		// pp.Println("req", req)
		beans, err := findBeans(db, "latest", req, bs.DIGEST_COLUMNS)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, beans)
	}
}

func createTrendingDigestsHandler(db *bs.Beansack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(BeansQueryRequest)
		// pp.Println("req", req)
		beans, err := findBeans(db, "trending", req, bs.DIGEST_COLUMNS)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, beans)
	}
}

func findBeans(db *bs.Beansack, trending_or_latest string, req BeansQueryRequest, columns_expr string) ([]bs.Bean, error) {
	var columns []string
	if columns_expr != "" {
		columns = []string{columns_expr}
	} else {
		columns = nil
	}
	var kinds []string
	if req.Kind != "" {
		kinds = []string{req.Kind}
	} else {
		kinds = nil
	}
	if trending_or_latest == "trending" {
		return db.QueryTrendingBeans(
			req.URLs,
			kinds, // only one kind
			req.Authors,
			req.Sources,
			req.CreatedSince,
			req.UpdatedSince,
			req.Categories,
			req.Regions,
			req.Entities,
			req.Embedding,
			req.MaxDistance,
			nil,
			nil,
			req.Limit,
			req.Offset,
			columns,
		)
	} else {
		return db.QueryLatestBeans(
			req.URLs,
			kinds,
			req.Authors,
			req.Sources,
			req.CreatedSince,
			req.Categories,
			req.Regions,
			req.Entities,
			req.Embedding,
			req.MaxDistance,
			nil,
			nil,
			req.Limit,
			req.Offset,
			columns,
		)
	}
}
