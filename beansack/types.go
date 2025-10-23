package beansack

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

type Float32Array []float32
type StringArray []string

type EmbeddingData struct {
	URL       string       `db:"url" bson:"url"`
	Embedding Float32Array `db:"embedding" bson:"embedding" json:"embedding"`
}

type TagData struct {
	URL string `db:"url"`
	Tag string `db:"tag"`
}

// type Bean struct {
// 	URL           string    `db:"url"`
// 	Kind          string    `db:"kind"`
// 	Title         string    `db:"title"`
// 	TitleLength   int       `db:"title_length" bson:"num_words_in_title"`
// 	Content       string    `db:"content"`
// 	ContentLength int       `db:"content_length" bson:"num_words_in_content"`
// 	Summary       string    `db:"summary"`
// 	SummaryLength int       `db:"summary_length" bson:"num_words_in_summary"`
// 	Author        string    `db:"author"`
// 	Source        string    `db:"source"`
// 	Created       time.Time `db:"created" bson:"created"`
// 	Collected     time.Time `db:"collected" bson:"collected"`
// }

type Bean struct {
	URL               string       `db:"url" json:"url,omitempty"`
	Kind              string       `db:"kind" json:"kind,omitempty"`
	Title             string       `db:"title" json:"title,omitempty"`
	TitleLength       int          `db:"title_length" bson:"num_words_in_title" json:"-"`
	Summary           string       `db:"summary" json:"summary,omitempty"`
	SummaryLength     int          `db:"summary_length" bson:"num_words_in_summary" json:"-"`
	Content           string       `db:"content" json:"content,omitempty"`
	ContentLength     int          `db:"content_length" bson:"num_words_in_content" json:"-"`
	RestrictedContent bool         `db:"restricted_content" bson:"is_scraped" json:"is_scraped,omitempty"`
	Author            string       `db:"author" json:"author,omitempty"`
	Source            string       `db:"source" json:"source,omitempty"`
	ImageUrl          string       `db:"image_url" bson:"image_url" json:"image_url,omitempty"`
	Created           time.Time    `db:"created" bson:"created" json:"created,omitempty"`
	Collected         time.Time    `db:"collected" bson:"collected" json:"-"`
	Embedding         Float32Array `db:"embedding" json:"embedding,omitempty"`
	Categories        StringArray  `db:"categories" json:"categories,omitempty"`
	Sentiments        StringArray  `db:"sentiments" json:"sentiments,omitempty"`
	Related           StringArray  `db:"related" json:"related,omitempty"`
	ClusterId         string       `db:"cluster_id" json:"cluster_id,omitempty"`
	ClusterSize       int          `db:"cluster_size" json:"cluster_size,omitempty"`
	Gist              string       `db:"gist" json:"gist,omitempty"`
	Regions           StringArray  `db:"regions" json:"regions,omitempty"`
	Entities          StringArray  `db:"entities" json:"entities,omitempty"`
	Updated           time.Time    `db:"updated" json:"updated,omitempty"` // last time some chatter was collected
	Likes             int          `db:"likes" json:"likes,omitempty"`
	Comments          int          `db:"comments" json:"comments,omitempty"`
	Subscribers       int          `db:"subscribers" json:"subscribers,omitempty"`
	Shares            int          `db:"shares" json:"shares,omitempty"`
	Distance          float64      `db:"distance" json:"-"`
}

type Chatter struct {
	ChatterURL  string    `db:"chatter_url" bson:"chatter_url" json:"chatter_url"`
	BeanURL     string    `db:"url" bson:"url" json:"url"`
	Source      string    `db:"source"`
	Forum       string    `db:"forum" bson:"group"`
	Collected   time.Time `db:"collected"`
	Likes       int       `db:"likes"`
	Comments    int       `db:"comments"`
	Subscribers int       `db:"subscribers"`
}

type BeanChatter struct {
	URL         string    `db:"url"`       // url of the bean
	Collected   time.Time `db:"collected"` // last time some chatter was collected
	Likes       int       `db:"likes"`
	Comments    int       `db:"comments"`
	Subscribers int       `db:"subscribers"`
	Shares      int       `db:"shares"`
}

type Publisher struct {
	Name        string `db:"name" bson:"site_name" json:"site_name,omitempty"`
	Description string `db:"description" bson:"description" json:"description,omitempty"`
	BaseURL     string `db:"base_url" bson:"site_base_url" json:"base_url,omitempty"`
	DomainName  string `db:"domain_name" bson:"source" json:"domain_name,omitempty"`
	Favicon     string `db:"favicon" bson:"site_favicon" json:"favicon,omitempty"`
	RSSFeed     string `db:"rss_feed" bson:"site_rss_feed" json:"rss_feed,omitempty"`
}

func (vec StringArray) Value() (driver.Value, error) {
	bytes, err := json.Marshal(vec)
	return driver.Value(string(bytes)), err
}

func (vec *StringArray) Scan(value interface{}) error {
	if value == nil {
		*vec = nil
		return nil
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

func (vec Float32Array) Value() (driver.Value, error) {
	if len(vec) == 0 {
		return driver.Value(nil), fmt.Errorf("vector cannot be nil or empty")
	}
	bytes, err := json.Marshal(vec)
	return driver.Value(string(bytes)), err
}

func (vec *Float32Array) Scan(value interface{}) error {
	if value == nil {
		*vec = nil
		return nil
	}

	switch value := value.(type) {
	case []interface{}:
		converted := make([]float32, len(value))
		for i, val := range value {
			switch v := val.(type) {
			case float64:
				converted[i] = float32(v)
			case float32:
				converted[i] = v
			case int:
				converted[i] = float32(v)
			default:
				return fmt.Errorf("unsupported array element type: %T", val)
			}
		}
		*vec = converted
		return nil
	case []float32:
		*vec = value
		return nil
	case []float64:
		converted := make([]float32, len(value))
		for i, v := range value {
			converted[i] = float32(v)
		}
		*vec = converted
		return nil
	case []int:
		converted := make([]float32, len(value))
		for i, val := range value {
			converted[i] = float32(val)
		}
		*vec = converted
		return nil
	case []byte:
		return json.Unmarshal(value, vec)
	case string:
		return json.Unmarshal([]byte(value), vec)
	default:
		return fmt.Errorf("unsupported type: %T", value)
	}
}
