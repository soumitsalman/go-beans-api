package main

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

const (
	NEWS      = "news"
	BLOG      = "blog"
	POST      = "post"
	GENERATED = "generated"
	COMMENT   = "comment"
)

type Vector []float32
type Tags []string

type EmbeddingData struct {
	URL       string `db:"url" bson:"url"`
	Embedding Vector `db:"embedding" bson:"embedding" json:"embedding"`
}

type TagData struct {
	URL string `db:"url"`
	Tag string `db:"tag"`
}

type Bean struct {
	URL           string    `db:"url"`
	Kind          string    `db:"kind"`
	Title         string    `db:"title"`
	TitleLength   int       `db:"title_length" bson:"num_words_in_title"`
	Content       string    `db:"content"`
	ContentLength int       `db:"content_length" bson:"num_words_in_content"`
	Summary       string    `db:"summary"`
	SummaryLength int       `db:"summary_length" bson:"num_words_in_summary"`
	Author        string    `db:"author"`
	Source        string    `db:"source"`
	Created       time.Time `db:"created" bson:"created"`
	Updated       time.Time `db:"updated" bson:"updated"`
	Collected     time.Time `db:"collected" bson:"collected"`
}

type BeanAggregate struct {
	Bean
	EmbeddingData
	ChatterAggregate
	Related    Tags   `db:"related"`
	Categories Tags   `db:"categories"`
	Sentiments Tags   `db:"sentiments"`
	Regions    Tags   `db:"regions"`
	Entities   Tags   `db:"entities"`
	Gist       string `db:"gist"`
}

type GeneratedBean struct {
	Bean
	Intro       string   `db:"intro"`
	Analysis    []string `db:"analysis"`
	Insights    []string `db:"insights"`
	Verdict     string   `db:"verdict"`
	Predictions []string `db:"predictions"`
}

type Chatter struct {
	ChatterURL  string    `db:"chatter_url" bson:"chatter_url"`
	BeanURL     string    `db:"bean_url" bson:"url"`
	Source      string    `db:"source"`
	Forum       string    `db:"forum" bson:"group"`
	Collected   time.Time `db:"collected"`
	Likes       int       `db:"likes"`
	Comments    int       `db:"comments"`
	Subscribers int       `db:"subscribers"`
}

type ChatterAggregate struct {
	URL           string    `db:"url"`            // url of the bean
	LastCollected time.Time `db:"last_collected"` // last time some chatter was collected
	Likes         int       `db:"total_likes"`
	Comments      int       `db:"total_comments"`
	Subscribers   int       `db:"total_subscribers"`
	Shares        int       `db:"total_shares"`
}

type Source struct {
	Name        string `db:"name" bson:"site_name"`
	Description string `db:"description" bson:"description"`
	BaseURL     string `db:"base_url" bson:"site_base_url"`
	DomainName  string `db:"domain_name" bson:"source"`
	Favicon     string `db:"favicon" bson:"site_favicon"`
	RSSFeed     string `db:"rss_feed" bson:"site_rss_feed"`
}

func (vec Tags) Value() (driver.Value, error) {
	bytes, err := json.Marshal(vec)
	return driver.Value(string(bytes)), err
}

func (vec *Tags) Scan(value interface{}) error {
	if value == nil {
		return fmt.Errorf("value cannot be nil")
	}
	switch value := value.(type) {
	case []interface{}:
		converted := make([]string, len(value))
		for i, val := range value {
			converted[i] = val.(string)
		}
		*vec = converted
	case []byte:
	case string:
		return json.Unmarshal([]byte(value), vec)
	default:
		return fmt.Errorf("unsupported type: %T", value)
	}
	return nil
}

func (vec Vector) Value() (driver.Value, error) {
	if len(vec) == 0 {
		return driver.Value(nil), fmt.Errorf("vector cannot be nil or empty")
	}
	bytes, err := json.Marshal(vec)
	return driver.Value(string(bytes)), err
}

func (vec *Vector) Scan(value interface{}) error {
	if value == nil {
		return fmt.Errorf("value cannot be nil")
	}
	switch value := value.(type) {
	case []interface{}:
		converted := make([]float32, len(value))
		for i, val := range value {
			converted[i] = val.(float32)
		}
		*vec = converted
	case []float32:
		*vec = value
		return nil
	case []float64:
	case []int:
		converted := make([]float32, len(value))
		for i, val := range value {
			converted[i] = float32(val)
		}
		*vec = converted
	case []byte:
	case string:
		return json.Unmarshal([]byte(value), vec)
	default:
		return fmt.Errorf("unsupported type: %T", value)
	}
	return nil
}
