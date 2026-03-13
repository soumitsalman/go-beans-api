package beansack

import (
	"time"
)

const (
	NEWS      = "news"
	BLOG      = "blog"
	POST      = "post"
	GENERATED = "generated"
	COMMENT   = "comment"
)

const (
	K_URL          = "url"
	K_KIND         = "kind"
	K_TITLE        = "title"
	K_SUMMARY      = "summary"
	K_CONTENT      = "content"
	K_AUTHOR       = "author"
	K_SOURCE       = "source"
	K_IMAGE_URL    = "image_url"
	K_CREATED      = "created"
	K_CATEGORIES   = "categories"
	K_SENTIMENTS   = "sentiments"
	K_REGIONS      = "regions"
	K_ENTITIES     = "entities"
	K_GIST         = "gist"
	K_EMBEDDING    = "embedding"
	K_RELATED      = "related"
	K_CLUSTER_ID   = "cluster_id"
	K_CLUSTER_SIZE = "cluster_size"
	K_LIKES        = "likes"
	K_COMMENTS     = "comments"
	K_SUBSCRIBERS  = "subscribers"
	K_SHARES       = "shares"
	K_TRENDSCORE   = "trend_score"
)

// Bean represents a single article or post indexed by Beansack.
// @Description Bean is the main content model returned by article endpoints. It contains
// identifying metadata (URL, Source), human-friendly fields (Title, Summary, Author),
// optional full `Content`, publishing timestamp (`Created`), and derived LLM fields
// used for search and classification: `Embedding` (vector), `Gist`, `Categories`,
// `Sentiments`, `Regions`, and `Entities`.
//
// Notes:
// - `Embedding` is a numeric vector used for semantic search and is omitted from JSON responses.
// - `Created` is encoded as a date-time string by the Swagger generator.
type Bean struct {
	URL        string    `db:"url" json:"url,omitempty"`
	Kind       string    `db:"kind" json:"content_type,omitempty"`
	Title      string    `db:"title" json:"title,omitempty"`
	Summary    string    `db:"summary" json:"summary,omitempty"`
	Content    string    `db:"content" json:"content,omitempty"`
	Author     string    `db:"author" json:"author,omitempty"`
	Source     string    `db:"source" json:"source,omitempty"`
	ImageUrl   string    `db:"image_url" json:"image_url,omitempty"`
	Created    time.Time `db:"created" json:"publish_date,omitempty,omitzero" swaggertype:"string" format:"date-time"`
	Embedding  []float32 `db:"embedding" json:"-"`
	Gist       string    `db:"gist" json:"-"`
	Categories []string  `db:"categories" json:"categories,omitempty"`
	Sentiments []string  `db:"sentiments" json:"sentiments,omitempty"`
	Regions    []string  `db:"regions" json:"regions,omitempty"`
	Entities   []string  `db:"entities" json:"entities,omitempty"`
}

// Chatter represents short-form discussion metadata associated with a Bean.
// @Description Chatter models a single social/forum mention of a Bean's URL and includes
// the mention URL (`ChatterURL`), the referenced `URL` (Bean URL), the `Source`/platform,
// optional `Forum`/group, collection timestamp (`Collected`), and engagement metrics
// (`Likes`, `Comments`, `Subscribers`).
type Chatter struct {
	ChatterURL  string    `db:"chatter_url" bson:"chatter_url" json:"chatter_url"`
	URL         string    `db:"url" bson:"url" json:"url"`
	Source      string    `db:"source" json:"source,omitempty"`
	Forum       string    `db:"forum" bson:"group" json:"forum,omitempty"`
	Collected   time.Time `db:"collected" json:"-" swaggertype:"string" format:"date-time"`
	Likes       int64     `db:"likes" json:"likes,omitempty"`
	Comments    int64     `db:"comments" json:"comments,omitempty"`
	Subscribers int64     `db:"subscribers" json:"subscribers,omitempty"`
}

// Publisher holds metadata about a content source (publisher).
// @Description Publisher contains identifying and descriptive information about a publisher
// or content source. It exposes the canonical `Source` id, `BaseURL`, optional `SiteName`,
// a human-friendly `Description`, and `Favicon`. `RSSFeed` and `Collected` are stored but
// not returned in JSON responses by default.
type Publisher struct {
	Source      string    `db:"source" json:"source,omitempty"`
	BaseURL     string    `db:"base_url" json:"source_base_url,omitempty"`
	SiteName    string    `db:"site_name" json:"source_site_name,omitempty"`
	Description string    `db:"description" json:"source_description,omitempty"`
	Favicon     string    `db:"favicon" json:"source_favicon,omitempty"`
	RSSFeed     string    `db:"rss_feed" json:"-"`
	Collected   time.Time `db:"collected" json:"-"`
}

// ChatterAggregate represents aggregated social engagement metrics for a Bean URL.
// @Description ChatterAggregate provides a summary of social traction for a Bean: the
// Bean `URL`, last `Collected` timestamp, and aggregate metrics `Likes`, `Comments`,
// `Subscribers`, and `Shares`. These fields are used to compute trend scores and
// to surface engagement in list APIs.
type ChatterAggregate struct {
	URL         string    `db:"url" json:"url,omitempty"` // url of the bean
	Collected   time.Time `db:"collected" json:"-"`       // last time some chatter was collected
	Likes       int64     `db:"likes" json:"likes,omitempty"`
	Comments    int64     `db:"comments" json:"comments,omitempty"`
	Subscribers int64     `db:"subscribers" json:"subscribers,omitempty"`
	Shares      int64     `db:"shares" json:"shares,omitempty"`
}

// BeanAggregate contains a `Bean` plus publisher metadata and aggregated analytics.
// @Description BeanAggregate composes a `Bean` with the publisher's display fields
// (BaseURL, SiteName, Description, Favicon) and aggregated social metrics
// (Likes, Comments, Subscribers, Shares). It also includes computed and analytical
// fields used by listing endpoints: `MergedTags` (computed union of categories/regions/entities),
// `Related` (related URLs), `ClusterId`/`ClusterSize`, `Updated` timestamp, `Distance`
// (for vector search), and `TrendScore`.
//
// Notes:
// - `MergedTags` is a computed field (db:"-") that consolidates tag-like fields for UI display.
// - Publisher `Source` remains on the embedded `Bean` and is the canonical source id.
type BeanAggregate struct {
	Bean
	// Computed tags merged from categories/regions/entities for display
	MergedTags []string `db:"-" json:"tags,omitempty"`

	// Publisher display fields
	BaseURL     string `db:"base_url" json:"source_base_url,omitempty"`
	SiteName    string `db:"site_name" json:"source_site_name,omitempty"`
	Description string `db:"description" json:"source_description,omitempty"`
	Favicon     string `db:"favicon" json:"source_favicon,omitempty"`

	// Aggregated social metrics
	Likes       int64 `db:"likes" json:"likes,omitempty"`
	Comments    int64 `db:"comments" json:"comments,omitempty"`
	Subscribers int64 `db:"subscribers" json:"subscribers,omitempty"`
	Shares      int64 `db:"shares" json:"shares,omitempty"`

	Related     []string  `db:"related" json:"related,omitempty"`
	ClusterId   string    `db:"cluster_id" json:"cluster_id,omitempty"`
	ClusterSize int64     `db:"cluster_size" json:"num_related,omitempty"`
	Updated     time.Time `db:"updated" json:"-"`
	Distance    float64   `db:"distance" json:"-"`
	TrendScore  float64   `db:"trend_score" json:"trend_score,omitempty"`
}
