package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// //////// STORE HANDLERS //////////
func createStoreBeansHandler(ds *Ducksack) gin.HandlerFunc {
	return func(c *gin.Context) {
		var beans []Bean
		if err := c.ShouldBindJSON(&beans); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ds.StoreBeans(beans)
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}

func createStoreEmbeddingsHandler(ds *Ducksack) gin.HandlerFunc {
	return func(c *gin.Context) {
		var embeddings []Bean
		if err := c.ShouldBindJSON(&embeddings); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ds.StoreEmbeddings(embeddings)
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}

func createStoreTagsHandler(ds *Ducksack) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tags []Bean
		if err := c.ShouldBindJSON(&tags); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ds.StoreTags(tags)
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}

func createStoreChatterHandler(ds *Ducksack) gin.HandlerFunc {
	return func(c *gin.Context) {
		var chatters []Chatter
		if err := c.ShouldBindJSON(&chatters); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ds.StoreChatters(chatters)
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}

func createStoreSourceHandler(ds *Ducksack) gin.HandlerFunc {
	return func(c *gin.Context) {
		var sources []Source
		if err := c.ShouldBindJSON(&sources); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ds.StoreSources(sources)
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}

// //////// DELETE HANDLERS ///////
type DeleteRequests struct {
	Where []string `json:"where" binding:"required"`
}

func validateDeleteRequest(c *gin.Context) {
	var req DeleteRequests
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Where) == 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "'where' field is required"})
		return
	}
	c.Set("req", req)
	c.Next()
}

func createDeleteBeansHandler(ds *Ducksack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(DeleteRequests)
		err := ds.DeleteBeans(req.Where...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}

func createDeleteChattersHandler(ds *Ducksack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(DeleteRequests)
		err := ds.DeleteChatters(req.Where...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}

func createDeleteSourcesHandler(ds *Ducksack) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.MustGet("req").(DeleteRequests)
		err := ds.DeleteSources(req.Where...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}
