package beansack

import (
	"context"
	"errors"
	"time"
)

// name of mandatory tables
const (
	BEANS            = "beans"
	PUBLISHERS       = "publishers"
	CHATTERS         = "chatters"
	BEAN_RELATIONS   = "bean_relations"
	FIXED_CATEGORIES = "fixed_categories"
	FIXED_SENTIMENTS = "fixed_sentiments"
)

const (
	CORE_BEAN_FIELDS                = "url, kind, title, summary, author, source, image_url, created, categories, sentiments, regions, entities"
	CORE_PUBLISHER_FIELDS           = "source, base_url, site_name, description, favicon"
	EXTENDED_BEAN_FIELDS            = "url, kind, title, summary, author, source, image_url, created, categories, sentiments, regions, entities, base_url, site_name, description, favicon, likes, comments, shares, related"
	UNRESTRICTED_CONTENT_CONDITIONS = "restricted_content IS NULL AND content IS NOT NULL"
	ORDER_BY_LATEST                 = "created DESC"
	ORDER_BY_TRENDING               = "trend_score DESC"
	ORDER_BY_DISTANCE               = "distance ASC"
)

var ErrNotImplemented = errors.New("method not implemented")

type Condition struct {
	URLs       []string
	Kind       string
	Created    time.Time
	Updated    time.Time
	Collected  time.Time
	Categories []string
	Regions    []string
	Entities   []string
	Tags       []string
	Sources    []string
	Embedding  []float32
	Distance   float64
	Extra      []string // CAUTION: This is a catch-all for any additional conditions. Use with care to avoid SQL injection.
}

type Pagination struct {
	Limit  int
	Offset int
}

type Beansack interface {
	QueryBeans(ctx context.Context, conditions Condition, page Pagination, columns []string) ([]BeanAggregate, error)
	QueryLatestBeans(ctx context.Context, conditions Condition, page Pagination, columns []string) ([]Bean, error)
	QueryTrendingBeans(ctx context.Context, conditions Condition, page Pagination, columns []string) ([]BeanTrend, error)
	QueryPublishers(ctx context.Context, conditions Condition, page Pagination, columns []string) ([]Publisher, error)
	QueryChatters(ctx context.Context, conditions Condition, page Pagination, columns []string) ([]Chatter, error)

	DistinctCategories(ctx context.Context, page Pagination) ([]string, error)
	DistinctSentiments(ctx context.Context, page Pagination) ([]string, error)
	DistinctEntities(ctx context.Context, page Pagination) ([]string, error)
	DistinctRegions(ctx context.Context, page Pagination) ([]string, error)
	DistinctSources(ctx context.Context, page Pagination) ([]string, error)

	CountRows(ctx context.Context, table string, conditions Condition) (int64, error)
	Close()
}
