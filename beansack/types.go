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
	// URL is the canonical URL of the article or post.
	URL string `db:"url" json:"url,omitempty"`
	// Kind is the content type, for example news, blog, post, generated, or comment.
	Kind string `db:"kind" json:"content_type,omitempty"`
	// Title is the human-readable headline or title of the content.
	Title string `db:"title" json:"title,omitempty"`
	// Summary is a short abstract or teaser used in listings and previews.
	Summary string `db:"summary" json:"summary,omitempty"`
	// Content is the full body text when the source content is available.
	Content string `db:"content" json:"content,omitempty"`
	// Author is the byline or attributed creator when available from the source.
	Author string `db:"author" json:"author,omitempty"`
	// Source is the canonical publisher identifier and matches Publisher.Source.
	Source string `db:"source" json:"source,omitempty"`
	// ImageUrl is the featured image or preview image associated with the content.
	ImageUrl string `db:"image_url" json:"image_url,omitempty"`
	// Created is the original publish timestamp of the article or post.
	Created time.Time `db:"created" json:"published_at,omitempty,omitzero" swaggertype:"string" format:"date-time"`
	// Embedding stores the semantic vector used for similarity search and is not returned in JSON.
	Embedding []float32 `db:"embedding" json:"-"`
	// Gist stores internal highlights extracted from the content and is not returned in JSON.
	Gist string `db:"gist" json:"-"`
	// Categories lists the inferred topics assigned to the content.
	Categories []string `db:"categories" json:"categories,omitempty"`
	// Sentiments lists inferred tones or sentiments expressed in the content.
	Sentiments []string `db:"sentiments" json:"sentiments,omitempty"`
	// Regions lists geographic regions mentioned in or associated with the content.
	Regions []string `db:"regions" json:"regions,omitempty"`
	// Entities lists named entities such as people, places, organizations, or products.
	Entities []string `db:"entities" json:"entities,omitempty"`
	// Computed tags merged from categories/regions/entities for display
	MergedTags []string `db:"-" json:"tags,omitempty"`
}

// Chatter represents short-form discussion metadata associated with a Bean.
// @Description Chatter models a single social/forum mention of a Bean's URL and includes
// the mention URL (`ChatterURL`), the referenced `URL` (Bean URL), the `Source`/platform,
// optional `Forum`/group, collection timestamp (`Collected`), and engagement metrics
// (`Likes`, `Comments`, `Subscribers`). Engagement counts represent cumulative lower-bound
// totals observed at collection time.
type Chatter struct {
	// ChatterURL is the URL of the social post, comment, or discussion item that mentions the Bean URL.
	ChatterURL string `db:"chatter_url" bson:"chatter_url" json:"chatter_url"`
	// URL is the referenced Bean URL that appeared in the social or forum mention.
	URL string `db:"url" bson:"url" json:"url"`
	// Source identifies the platform or publisher where the chatter was collected.
	Source string `db:"source" json:"source,omitempty"`
	// Forum is the community, group, subreddit, page, or forum where the mention was found.
	Forum string `db:"forum" bson:"group" json:"forum,omitempty"`
	// Collected is when the chatter metrics were collected from the external platform.
	Collected time.Time `db:"collected" json:"-" swaggertype:"string" format:"date-time"`
	// Likes is the cumulative lower-bound like or upvote count captured for the mention.
	Likes int64 `db:"likes" json:"likes,omitempty"`
	// Comments is the cumulative lower-bound reply or comment count captured for the mention.
	Comments int64 `db:"comments" json:"comments,omitempty"`
	// Subscribers is the cumulative lower-bound audience or follower count for the forum/community.
	Subscribers int64 `db:"subscribers" json:"subscribers,omitempty"`
}

// Publisher holds metadata about a content source (publisher).
// @Description Publisher contains identifying and descriptive information about a publisher
// or content source. It exposes the canonical `Source` id, `BaseURL`, optional `SiteName`,
// a human-friendly `Description`, and `Favicon`. `RSSFeed` and `Collected` are stored but
// not returned in JSON responses by default.
type Publisher struct {
	// Source is the canonical publisher identifier and matches Bean.Source values.
	Source string `db:"source" json:"source,omitempty"`
	// BaseURL is the publisher's primary site URL.
	BaseURL string `db:"base_url" json:"source_base_url,omitempty"`
	// SiteName is the human-readable display name of the publisher.
	SiteName string `db:"site_name" json:"source_site_name,omitempty"`
	// Description is a short description of the publisher or content source.
	Description string `db:"description" json:"source_description,omitempty"`
	// Favicon is the URL of the publisher favicon or brand icon.
	Favicon string `db:"favicon" json:"source_favicon,omitempty"`
	// RSSFeed stores the publisher feed URL and is omitted from JSON responses.
	RSSFeed string `db:"rss_feed" json:"-"`
	// Collected is when the publisher metadata was last collected and is omitted from JSON responses.
	Collected time.Time `db:"collected" json:"-" swaggertype:"string" format:"date-time"`
}

// ChatterAggregate represents aggregated social engagement metrics for a Bean URL.
// @Description ChatterAggregate provides a summary of social traction for a Bean: the
// Bean `URL`, last `Collected` timestamp, and aggregate metrics `Likes`, `Comments`,
// `Subscribers`, and `Shares`. These fields are used to compute trend scores and
// to surface engagement in list APIs.
type ChatterAggregate struct {
	// URL is the Bean URL for which aggregate chatter metrics were computed.
	URL string `db:"url" json:"url,omitempty"`
	// Collected is the latest timestamp when any contributing chatter record was collected.
	Collected time.Time `db:"collected" json:"-" swaggertype:"string" format:"date-time"`
	// Likes is the aggregate number of likes or upvotes across collected chatter records.
	Likes int64 `db:"likes" json:"likes,omitempty"`
	// Comments is the aggregate number of replies or comments across collected chatter records.
	Comments int64 `db:"comments" json:"comments,omitempty"`
	// Subscribers is the aggregate audience size associated with contributing chatter records.
	Subscribers int64 `db:"subscribers" json:"subscribers,omitempty"`
	// Shares is the aggregate number of reposts, retweets, or share-like actions.
	Shares int64 `db:"shares" json:"shares,omitempty"`
}

// BeanTrend contains a `Bean` plus trend analytics.
// @Description BeanTrend composes a `Bean` with aggregated social metrics (Likes, Comments, Subscribers, Shares, Related, Updated timestamp, TrendScore).
//
// Notes:
// - `MergedTags` is a computed field (db:"-") that consolidates tag-like fields for UI display.
type BeanTrend struct {
	// Bean embeds the primary content record returned by article endpoints.
	Bean

	// Likes is the aggregate number of likes or upvotes associated with this Bean.
	Likes int64 `db:"likes" json:"likes,omitempty"`
	// Comments is the aggregate number of replies or comments associated with this Bean.
	Comments int64 `db:"comments" json:"comments,omitempty"`
	// Subscribers is the aggregate audience size associated with this Bean's chatter.
	Subscribers int64 `db:"subscribers" json:"subscribers,omitempty"`
	// Shares is the aggregate number of reposts or share-like actions associated with this Bean.
	Shares int64 `db:"shares" json:"shares,omitempty"`
	// Related lists URLs of semantically or editorially related Beans.
	Related int64 `db:"related" json:"related,omitempty"`
	// Updated is when aggregate analytics were last refreshed and is omitted from JSON responses.
	Updated time.Time `db:"updated" json:"-" swaggertype:"string" format:"date-time"`
	// TrendScore is the computed ranking score used to order trending results.
	TrendScore float64 `db:"trend_score" json:"trend_score,omitempty"`
}

// BeanAggregate contains a `BeanTrend` plus publisher metadata.
// @Description BeanAggregate composes a `BeanTrend` with the publisher's display fields
// (BaseURL, SiteName, Description, Favicon).
//
// Notes:
// - Publisher `Source` remains on the embedded `Bean` and is the canonical source id.
type BeanAggregate struct {
	// Bean embeds the primary content record returned by article endpoints.
	BeanTrend

	// BaseURL is the publisher's primary site URL copied onto aggregate results for convenience.
	BaseURL string `db:"base_url" json:"source_base_url,omitempty"`
	// SiteName is the human-readable name of the publisher copied onto aggregate results.
	SiteName string `db:"site_name" json:"source_site_name,omitempty"`
	// Description is the publisher description copied onto aggregate results.
	Description string `db:"description" json:"source_description,omitempty"`
	// Favicon is the publisher favicon URL copied onto aggregate results.
	Favicon string `db:"favicon" json:"source_favicon,omitempty"`
}
