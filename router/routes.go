package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	bs "github.com/soumitsalman/gobeansack/beansack"
)

const (
	Name            = "Beans API & MCP"
	Version         = "0.1"
	Description     = "Beans is an intelligent news & blogs aggregation and search service that curates fresh content from RSS feeds using AI-powered natural language queries and filters."
	DefaultAccuracy = 0.75
	DefaultLimit    = 16
	MinLimit        = 1
	MaxLimit        = 100
	FaviconPath     = "app/assets/images/beans.png"
)

var (
	processedItems      = []string{"gist IS NOT NULL", "embedding IS NOT NULL"}
	unrestrictedContent = []string{"restricted_content IS NULL", "content IS NOT NULL"}
	coreBeanFields      = []string{"url", "kind", "title", "summary", "author", "source", "image_url", "created", "categories", "sentiments", "regions", "entities"}
	extendedBeanFields  = append(append([]string{}, coreBeanFields...), "content")
	corePublisherFields = []string{"source", "base_url", "site_name", "description", "favicon"}
)

type Embedder interface {
	EmbedQuery(ctx context.Context, query string) ([]float32, error)
}

type RouterDeps struct {
	DB       bs.Beansack
	Embedder Embedder
	APIKeys  map[string]string
}

type HealthOutput struct {
	Body struct {
		Status string `json:"status"`
	}
}

type TagsInput struct {
	Offset int `query:"offset" default:"0" minimum:"0"`
	Limit  int `query:"limit" default:"16" minimum:"1" maximum:"100"`
}

type StringListOutput struct {
	Body []string
}

type ArticlesInput struct {
	Q              string    `query:"q" minLength:"3" maxLength:"512"`
	Acc            float64   `query:"acc" default:"0.75" minimum:"0" maximum:"1"`
	Kind           string    `query:"kind" enum:"news,blog"`
	Tags           []string  `query:"tags"`
	Sources        []string  `query:"sources"`
	PublishedSince time.Time `query:"published_since" format:"date-time"`
	TrendingSince  time.Time `query:"trending_since" format:"date-time"`
	WithContent    bool      `query:"with_content" default:"false"`
	Limit          int       `query:"limit" default:"16" minimum:"1" maximum:"100"`
	Offset         int       `query:"offset" default:"0" minimum:"0"`
}

type BeansOutput struct {
	Body []bs.Bean
}

type PublishersInput struct {
	Sources []string `query:"sources"`
	Limit   int      `query:"limit" default:"16" minimum:"1" maximum:"100"`
	Offset  int      `query:"offset" default:"0" minimum:"0"`
}

type PublishersOutput struct {
	Body []bs.Publisher
}

func InitializeRoutes(deps RouterDeps) http.Handler {
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig(Name, Version))
	api.OpenAPI().Info.Description = Description

	registerHealth(api)
	registerFavicon(api)
	registerTags(api, deps)
	registerArticles(api, deps)
	registerPublishers(api, deps)

	return authMiddleware(deps.APIKeys, mux)
}

func registerHealth(api huma.API) {
	huma.Get(api, "/health", func(ctx context.Context, _ *struct{}) (*HealthOutput, error) {
		out := &HealthOutput{}
		out.Body.Status = "alive"
		return out, nil
	})
}

func registerFavicon(api huma.API) {
	huma.Get(api, "/favicon.ico", func(ctx context.Context, _ *struct{}) (*huma.StreamResponse, error) {
		if _, err := os.Stat(FaviconPath); err != nil {
			return nil, huma.Error404NotFound("favicon not found")
		}
		path := filepath.Clean(FaviconPath)
		stream := func(ctx huma.Context) {
			http.ServeFile(ctx, ctx.Request(), path)
		}
		return &huma.StreamResponse{
			Body: stream,
		}, nil
	})
}

func registerTags(api huma.API, deps RouterDeps) {
	huma.Get(api, "/tags/categories", func(ctx context.Context, input *TagsInput) (*StringListOutput, error) {
		data, err := deps.DB.DistinctCategories(input.Limit, input.Offset)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to query categories", err)
		}
		return &StringListOutput{Body: data}, nil
	})
	huma.Get(api, "/tags/entities", func(ctx context.Context, input *TagsInput) (*StringListOutput, error) {
		data, err := deps.DB.DistinctEntities(input.Limit, input.Offset)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to query entities", err)
		}
		return &StringListOutput{Body: data}, nil
	})
	huma.Get(api, "/tags/regions", func(ctx context.Context, input *TagsInput) (*StringListOutput, error) {
		data, err := deps.DB.DistinctRegions(input.Limit, input.Offset)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to query regions", err)
		}
		return &StringListOutput{Body: data}, nil
	})
}

func registerArticles(api huma.API, deps RouterDeps) {
	huma.Get(api, "/articles/latest", func(ctx context.Context, input *ArticlesInput) (*BeansOutput, error) {
		embedding, distance, kind, err := vectorArgs(ctx, deps.Embedder, input.Q, input.Acc, input.Kind)
		if err != nil {
			return nil, err
		}
		var published *time.Time
		if !input.PublishedSince.IsZero() {
			published = &input.PublishedSince
		}
		conditions := processedItems
		columns := coreBeanFields
		if input.WithContent {
			conditions = append([]string{}, unrestrictedContent...)
			conditions = append(conditions, processedItems...)
			columns = extendedBeanFields
		}
		items, err := deps.DB.QueryLatestBeans(kind, published, nil, nil, nil, nil, input.Tags, input.Sources, embedding, distance, conditions, input.Limit, input.Offset, columns)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to query latest articles", err)
		}
		return &BeansOutput{Body: items}, nil
	})

	huma.Get(api, "/articles/trending", func(ctx context.Context, input *ArticlesInput) (*BeansOutput, error) {
		embedding, distance, kind, err := vectorArgs(ctx, deps.Embedder, input.Q, input.Acc, input.Kind)
		if err != nil {
			return nil, err
		}
		var trending *time.Time
		if !input.TrendingSince.IsZero() {
			trending = &input.TrendingSince
		}
		conditions := processedItems
		columns := coreBeanFields
		if input.WithContent {
			conditions = append([]string{}, unrestrictedContent...)
			conditions = append(conditions, processedItems...)
			columns = extendedBeanFields
		}
		items, err := deps.DB.QueryTrendingBeans(kind, trending, nil, nil, nil, nil, input.Tags, input.Sources, embedding, distance, conditions, input.Limit, input.Offset, columns)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to query trending articles", err)
		}
		return &BeansOutput{Body: items}, nil
	})
}

func registerPublishers(api huma.API, deps RouterDeps) {
	huma.Get(api, "/publishers", func(ctx context.Context, input *PublishersInput) (*PublishersOutput, error) {
		if len(input.Sources) == 0 {
			return nil, huma.Error400BadRequest("sources is required")
		}
		items, err := deps.DB.QueryPublishers(nil, nil, input.Sources, nil, input.Limit, input.Offset, corePublisherFields)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to query publishers", err)
		}
		return &PublishersOutput{Body: items}, nil
	})

	huma.Get(api, "/publishers/sources", func(ctx context.Context, input *TagsInput) (*StringListOutput, error) {
		items, err := deps.DB.DistinctPublishers(input.Limit, input.Offset)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to query publisher IDs", err)
		}
		return &StringListOutput{Body: items}, nil
	})
}

func vectorArgs(ctx context.Context, embedder Embedder, query string, acc float64, kindStr string) ([]float32, float64, *string, error) {
	var kind *string
	if kindStr != "" {
		v := strings.ToLower(kindStr)
		if v != bs.NEWS && v != bs.BLOG {
			return nil, 0, nil, huma.Error400BadRequest("kind must be either news or blog")
		}
		kind = &v
	}

	if strings.TrimSpace(query) == "" {
		return nil, 0, kind, nil
	}
	if embedder == nil {
		return nil, 0, nil, huma.Error500InternalServerError("embedder is not configured", errors.New("missing embedder"))
	}
	embedding, err := embedder.EmbedQuery(ctx, query)
	if err != nil {
		return nil, 0, nil, huma.Error500InternalServerError("failed to embed query", err)
	}
	return embedding, 1 - acc, kind, nil
}

func authMiddleware(apiKeys map[string]string, next http.Handler) http.Handler {
	if len(apiKeys) == 0 {
		return next
	}
	public := map[string]bool{"/health": true, "/favicon.ico": true}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if public[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		for header, value := range apiKeys {
			if r.Header.Get(header) == value {
				next.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Invalid API Key"})
	})
}

func ParseAPIKeys(raw string) map[string]string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	result := map[string]string{}
	for _, pair := range strings.Split(raw, ";") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}
		header := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if header != "" && value != "" {
			result[header] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func RequiredEnv(name string) (string, error) {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return v, nil
}
