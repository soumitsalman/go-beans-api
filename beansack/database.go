package beansack

import (
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

var ErrNotImplemented = errors.New("method not implemented")

type QueryConditions struct {
	Urls       []string
	Kind       string
	Created    *time.Time
	Updated    *time.Time
	Collected  *time.Time
	Categories []string
	Regions    []string
	Entities   []string
	Tags       []string
	Sources    []string
	Embedding  []float32
	Distance   float64
	Extra      []string // CAUTION: This is a catch-all for any additional conditions. Use with care to avoid SQL injection.
}

type QueryPage struct {
	Limit  int
	Offset int
}

type Beansack interface {
	QueryLatestBeans(conditions QueryConditions, page QueryPage, columns []string) ([]Bean, error)
	QueryTrendingBeans(conditions QueryConditions, page QueryPage, columns []string) ([]Bean, error)
	QueryPublishers(conditions QueryConditions, page QueryPage, columns []string) ([]Publisher, error)
	QueryChatters(conditions QueryConditions, page QueryPage, columns []string) ([]Chatter, error)

	DistinctCategories(page QueryPage) ([]string, error)
	DistinctSentiments(page QueryPage) ([]string, error)
	DistinctEntities(page QueryPage) ([]string, error)
	DistinctRegions(page QueryPage) ([]string, error)
	DistinctSources(page QueryPage) ([]string, error)

	CountRows(table string, conditions QueryConditions) (int64, error)
	Close()
}
