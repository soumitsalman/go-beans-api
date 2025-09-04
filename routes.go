package main

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

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

func initRoutes(ds *Ducksack) *gin.Engine {
	// vector_query_throttle := make(chan int, max(1, runtime.NumCPU()>>1))
	vector_query_throttle := make(chan int, 2)

	r := gin.Default()
	// Health check endpoint - no auth required
	r.GET("/health", healthCheckHandler)

	// NEWS API ENDPOINTS
	// everything sorted by created_at DESC
	public := r.Group("/public/beans", validateQueryRequest)
	{
		public.GET("/latest", createLatestBeansHandler(ds, vector_query_throttle))
		public.GET("/exists", createExistsHandler(ds))
		public.GET("/related", createRelatedBeansHandler(ds))
		public.GET("/regions", createRegionsHandler(ds))
		public.GET("/entities", createEntitiesHandler(ds))
		public.GET("/categories", createCategoriesHandler(ds))
		public.GET("/sources", createSourcesHandler(ds))
	}

	// REGISTERED APPLICATION ENDPOINTS
	// everything sorted by trending DESC
	privileged := r.Group("/privileged/beans", createAuthVerificationHandler("PRIVILEGED_KEY"), validateQueryRequest)
	{
		privileged.GET("/latest/contents", createLatestContentsHandler(ds, vector_query_throttle))
		privileged.GET("/trending", createTrendingBeansHandler(ds, vector_query_throttle))
		privileged.GET("/trending/tags", createTrendingTagsHandler(ds, vector_query_throttle))
		privileged.GET("/trending/embeddings", createTrendingEmbeddingsHandler(ds, vector_query_throttle))
	}

	// CONTRIBUTOR ENDPOINTS
	publisher := r.Group("/publisher", createAuthVerificationHandler("PUBLISHER_KEY"))
	{
		publisher.POST("/beans", createStoreBeansHandler(ds))
		publisher.POST("/beans/tags", createStoreTagsHandler(ds))
		publisher.POST("/beans/embeddings", createStoreEmbeddingsHandler(ds))
		publisher.POST("/chatters", createStoreChatterHandler(ds))
		publisher.POST("/sources", createStoreSourceHandler(ds))
		publisher.DELETE("/beans", validateDeleteRequest, createDeleteBeansHandler(ds))
		publisher.DELETE("/chatters", validateDeleteRequest, createDeleteChattersHandler(ds))
		publisher.DELETE("/sources", validateDeleteRequest, createDeleteSourcesHandler(ds))
	}

	// ADMIN ENDPOINTS
	admin := r.Group("/admin", createAuthVerificationHandler("ADMIN_KEY"))
	{
		admin.POST("/execute", createAdminCommandHandler(ds))
	}

	return r
}
