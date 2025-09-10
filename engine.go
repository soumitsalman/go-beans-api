package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

type BeanSackEngine struct {
	ds           *BeanSack
	throttler    chan int
	router       *gin.Engine
	refresh_time int
}

func NewEngine(datapath string, init_sql string, vector_dimensions int, related_eps float64, max_concurrent_queries int, refresh_time int) *BeanSackEngine {
	engine := BeanSackEngine{
		ds:           NewDuckSack(datapath, init_sql, vector_dimensions, related_eps),
		throttler:    make(chan int, max_concurrent_queries),
		router:       gin.Default(),
		refresh_time: refresh_time,
	}
	initRoutes(&engine)
	return &engine
}

func (eng *BeanSackEngine) Run(addr string) error {
	ticker := time.NewTicker(time.Duration(eng.refresh_time) * time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for {
			select {
			case <-ticker.C:
				eng.ds.Refresh()

			case <-ctx.Done():
				ticker.Stop()
				cancel()
				return
			}
		}
	}()
	return eng.router.Run(addr)
}

func (eng *BeanSackEngine) Close() {
	eng.ds.Close()
}

func initRoutes(eng *BeanSackEngine) {
	// Health check endpoint - no auth required
	eng.router.GET("/health", healthCheckHandler)
	enter_throttle := createEnterThrottle(eng.throttler)
	exit_throttle := createExitThrottle(eng.throttler)

	// NEWS API ENDPOINTS
	// everything sorted by created_at DESC
	public := eng.router.Group("/public/beans", validateQueryRequest)
	{
		// these are composite queries
		public.GET("/latest", createLatestBeansHandler(eng.ds))
		public.GET("/trending", createTrendingBeansHandler(eng.ds))
		public.GET("/trending/digests", createTrendingDigestsHandler(eng.ds))
		// these are query by URL
		public.GET("/exists", createExistsHandler(eng.ds))
		public.GET("/embeddings", createEmbeddingsHandler(eng.ds))
		public.GET("/related", createRelatedBeansHandler(eng.ds))
		public.GET("/categories", createCategoriesHandler(eng.ds))
		public.GET("/sentiments", createSentimentsHandler(eng.ds))
		public.GET("/regions", createRegionsHandler(eng.ds))
		public.GET("/entities", createEntitiesHandler(eng.ds))
		public.GET("/sources", createSourcesHandler(eng.ds))
	}

	// REGISTERED APPLICATION ENDPOINTS
	// everything sorted by trending DESC
	privileged := eng.router.Group("/privileged/beans", createAuthVerificationHandler("PRIVILEGED_KEY"), validateQueryRequest, enter_throttle)
	{
		privileged.GET("/untagged", createLatestUntaggedBeansHandler(eng.ds), exit_throttle)
	}

	// PUBLISHER ENDPOINTS
	publisher := eng.router.Group("/publisher", createAuthVerificationHandler("PUBLISHER_KEY"))
	{
		publisher.POST("/beans", createStoreBeansHandler(eng.ds))
		publisher.POST("/beans/tags", createStoreTagsHandler(eng.ds))
		publisher.POST("/beans/embeddings", createStoreEmbeddingsHandler(eng.ds))
		publisher.POST("/chatters", createStoreChatterHandler(eng.ds))
		publisher.POST("/sources", createStoreSourceHandler(eng.ds))
		publisher.DELETE("/beans", validateDeleteRequest, createDeleteBeansHandler(eng.ds))
		publisher.DELETE("/chatters", validateDeleteRequest, createDeleteChattersHandler(eng.ds))
		publisher.DELETE("/sources", validateDeleteRequest, createDeleteSourcesHandler(eng.ds))
	}

	// ADMIN ENDPOINTS
	admin := eng.router.Group("/admin", createAuthVerificationHandler("ADMIN_KEY"))
	{
		admin.POST("/execute", createAdminCommandHandler(eng.ds))
	}
}

func healthCheckHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

func createAuthVerificationHandler(expectedKeyName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-KEY")
		expectedKey := os.Getenv(expectedKeyName)

		if expectedKey == "" || apiKey == expectedKey {
			c.Next()
		} else {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
		}
	}
}

func createEnterThrottle(throttler chan int) gin.HandlerFunc {
	return func(c *gin.Context) {
		throttler <- 1
		c.Next()
	}
}

func createExitThrottle(throttler chan int) gin.HandlerFunc {
	return func(c *gin.Context) {
		<-throttler
		c.Next()
	}
}
