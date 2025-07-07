package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/marcboeker/go-duckdb"
	_ "github.com/marcboeker/go-duckdb"
	_ "github.com/mattn/go-sqlite3"
)

const (
	BEANS           = "beans"
	BEAN_EMBEDDINGS = "bean_embeddings"
	BEAN_CLUSTERS   = "bean_clusters"
	BEAN_CATEGORIES = "bean_categories"
	BEAN_SENTIMENTS = "bean_sentiments"
	BEAN_GISTS      = "bean_gists"
	BEAN_REGIONS    = "bean_regions"
	BEAN_ENTITIES   = "bean_entities"
	CHATTERS        = "chatters"
	SOURCES         = "sources"
	CATEGORIES      = "categories"
	SENTIMENTS      = "sentiments"
)

type Ducksack struct {
	connector *duckdb.Connector
	db        *sql.DB
	query     *sqlx.DB
	dim       int
}

////////// INITIALIZE DATABASE //////////

func NewDucksack(datapath string, initsql string, vectordim int) *Ducksack {
	conn, err := duckdb.NewConnector(fmt.Sprintf("%s?threads=%d", datapath, max(1, runtime.NumCPU()-1)), nil)
	noerror(err)

	// open connection
	db := sql.OpenDB(conn)
	if initsql != "" {
		_, err = db.Exec(initsql)
		noerror(err)
	}

	return &Ducksack{
		connector: conn,
		db:        db,
		query:     sqlx.NewDb(db, "duckdb"),
		dim:       vectordim,
	}
}

////////// STORING FUNCTIONS //////////

// func (ds *Ducksack) getAppender(table string) *duckdb.Appender {
// 	conn, err := ds.connector.Connect(context.Background())
// 	noerror(err)
// 	appender, err := duckdb.NewAppenderFromConn(conn, "", table)
// 	noerror(err)
// 	return appender
// }

func appendToTable[T any](ds *Ducksack, table string, data []T, getfieldvalues func(item T) []driver.Value) int {
	conn, err := ds.connector.Connect(context.Background())
	noerror(err)
	appender, err := duckdb.NewAppenderFromConn(conn, "", table)
	noerror(err)
	defer appender.Close()
	count := 0
	for _, item := range data {
		if err := appender.AppendRow(getfieldvalues(item)...); err != nil {
			log.Println(err)
		} else {
			count++
		}
	}
	return count
}

func prepareBeans(beans []Bean) []Bean {
	now := time.Now()
	for i := range beans {
		if beans[i].Created.IsZero() {
			beans[i].Created = now
		}
		if beans[i].Updated.IsZero() {
			beans[i].Updated = now
		}
		if beans[i].Collected.IsZero() {
			beans[i].Collected = now
		}
		if beans[i].TitleLength == 0 {
			beans[i].TitleLength = len(strings.Fields(beans[i].Title))
		}
		if beans[i].ContentLength == 0 {
			beans[i].ContentLength = len(strings.Fields(beans[i].Content))
		}
		if beans[i].SummaryLength == 0 {
			beans[i].SummaryLength = len(strings.Fields(beans[i].Summary))
		}
	}
	return beans
}

func (ds *Ducksack) StoreBeans(beans []Bean) int {
	beans = prepareBeans(beans)
	return appendToTable(ds, BEANS, beans, func(bean Bean) []driver.Value {
		return []driver.Value{bean.URL, bean.Kind, bean.Title, bean.TitleLength, bean.Content, bean.ContentLength, bean.Summary, bean.SummaryLength, bean.Author, bean.Source, bean.Created, bean.Collected}
	})
}

func (ds *Ducksack) StoreEmbeddings(embeddings []EmbeddingData) int {
	return appendToTable(ds, BEAN_EMBEDDINGS, embeddings, func(embedding EmbeddingData) []driver.Value {
		return []driver.Value{embedding.URL, embedding.Embedding}
	})
}

func (ds *Ducksack) StoreTags(tags []TagData, tag_table string) int {
	return appendToTable(ds, tag_table, tags, func(tag TagData) []driver.Value {
		return []driver.Value{tag.URL, tag.Tag}
	})
}

func prepareChatters(chatters []Chatter) []Chatter {
	now := time.Now()
	for i := range chatters {
		if chatters[i].Collected.IsZero() {
			chatters[i].Collected = now
		}
	}
	return chatters
}

func (ds *Ducksack) StoreChatters(chatters []Chatter) int {
	chatters = prepareChatters(chatters)
	return appendToTable(ds, CHATTERS, chatters, func(chatter Chatter) []driver.Value {
		return []driver.Value{chatter.ChatterURL, chatter.BeanURL, chatter.Collected, chatter.Source, chatter.Forum, chatter.Likes, chatter.Comments, chatter.Subscribers}
	})
}

func (ds *Ducksack) StoreSources(sources []Source) int {
	return appendToTable(ds, SOURCES, sources, func(source Source) []driver.Value {
		return []driver.Value{source.Name, source.Description, source.BaseURL, source.DomainName, source.Favicon, source.RSSFeed}
	})
}

////////// QUERY WITH SCALAR MATCHING //////////

func mustIn(query string, args ...any) (string, []any) {
	query, args, err := sqlx.In(query, args...)
	noerror(err)
	return query, args
}

func mustSelect[T any](ds *Ducksack, query string, args ...any) []T {
	var data []T
	noerror(ds.query.Select(&data, query, args...))
	return data
}

func (ds *Ducksack) Exists(urls []string) []string {
	query, args := mustIn("SELECT url FROM beans WHERE url IN (?)", urls)
	return mustSelect[string](ds, query, args...)
}

func (ds *Ducksack) QueryBeans(urls []string) []Bean {
	query, args := mustIn("SELECT * FROM beans WHERE url IN (?)", urls)
	return mustSelect[Bean](ds, query, args...)
}

func (ds *Ducksack) QueryBeanEmbeddings(urls []string) []EmbeddingData {
	query, args := mustIn("SELECT * FROM bean_embeddings WHERE url IN (?)", urls)
	return mustSelect[EmbeddingData](ds, query, args...)
}

const _SQL_VECTOR_SEARCH_BEANS = `
SELECT * FROM bean_embeddings
ORDER BY array_cosine_distance(embedding, ?::FLOAT[%d])
LIMIT ?
`

func (ds *Ducksack) VectorSearchBeans(embedding []float32, limit int) []EmbeddingData {
	return mustSelect[EmbeddingData](
		ds,
		fmt.Sprintf(_SQL_VECTOR_SEARCH_BEANS, len(embedding)),
		Vector(embedding),
		limit,
	)
}

func (ds *Ducksack) QueryChatters(urls []string) []Chatter {
	query, args := mustIn("SELECT * FROM chatters WHERE bean_url IN (?) ORDER BY collected DESC", urls)
	return mustSelect[Chatter](ds, query, args...)
}

func (ds *Ducksack) QueryChatterAggregates(urls []string) []ChatterAggregate {
	query, args := mustIn("SELECT * FROM chatter_aggregates WHERE url IN (?)", urls)
	return mustSelect[ChatterAggregate](ds, query, args...)
}

// first take the chatters ONLY for the filtered urls
// then take the current chatters and group by id
// then then add/agg per bean
// take the ones that got updated in last 1 day
// take the chatters from 1 day ago per id
// then aggregate per bean
// then subtract
const _SQL_QUERY_CHATTER_UPDATES = `
WITH 
filtered_chatters AS (
    SELECT * FROM chatters WHERE bean_url IN (?)
),
current_agg AS (
	SELECT
        bean_url,
        MAX(collected) as collected,
        SUM(likes) as likes,
        SUM(comments) as comments,
        SUM(subscribers) as subscribers,
        COUNT(chatter_url) as shares,

    FROM (
		SELECT
			chatter_url,
			FIRST(bean_url) as bean_url,
			MAX(collected) as collected,
			MAX(likes) as likes,
			MAX(comments) as comments,
			MAX(subscribers) as subscribers
		FROM filtered_chatters
		GROUP BY chatter_url
	)
    GROUP BY bean_url
),
before_agg AS (
	SELECT
        bean_url,
        MAX(collected) as collected,
        SUM(likes) as likes,
        SUM(comments) as comments,
        SUM(subscribers) as subscribers,
        COUNT(chatter_url) as shares
    FROM (
		SELECT
			chatter_url,
			FIRST(bean_url) as bean_url,
			MAX(collected) as collected,
			MAX(likes) as likes,
			MAX(comments) as comments,
			MAX(subscribers) as subscribers
		FROM filtered_chatters
		WHERE collected + INTERVAL 1 DAY < CURRENT_TIMESTAMP
		GROUP BY chatter_url
	)
    GROUP BY bean_url
)
SELECT
	ca.bean_url as url,
	ca.collected as last_collected,
	COALESCE(ca.likes, 0) - COALESCE(ba.likes, 0) as total_likes,
	COALESCE(ca.comments, 0) - COALESCE(ba.comments, 0) as total_comments,
	COALESCE(ca.subscribers, 0) - COALESCE(ba.subscribers, 0) as total_subscribers,
	COALESCE(ca.shares, 0) - COALESCE(ba.shares, 0) as total_shares
FROM current_agg ca
LEFT JOIN before_agg ba
ON ca.bean_url = ba.bean_url
WHERE 
	ca.collected + INTERVAL 1 day >= CURRENT_TIMESTAMP AND
	(total_likes > 0 OR total_comments > 0 OR total_subscribers > 0 OR total_shares > 0);
`

func (ds *Ducksack) QueryChatterUpdates(urls []string) []ChatterAggregate {
	query, args := mustIn(_SQL_QUERY_CHATTER_UPDATES, urls)
	return mustSelect[ChatterAggregate](ds, query, args...)
}

////////// QUERY WITH FUZZY MATCHING //////////

const _SQL_MATCH_CATEGORIES = `
SELECT url, LIST(category) AS categories  
FROM top3_categories 
WHERE url IN (?) 
GROUP BY url;
`

func (ds *Ducksack) MatchCategories(urls []string) []BeanAggregate {
	query, args, err := sqlx.In(_SQL_MATCH_CATEGORIES, urls)
	noerror(err)
	var categories []BeanAggregate
	noerror(ds.query.Select(&categories, query, args...))
	return categories
}

const _SQL_MATCH_SENTIMENTS = `
SELECT url, LIST(sentiment) AS sentiments 
FROM top3_sentiments 
WHERE url IN (?) 
GROUP BY url;
`

func (ds *Ducksack) MatchSentiments(urls []string) []BeanAggregate {
	query, args, err := sqlx.In(_SQL_MATCH_SENTIMENTS, urls)
	noerror(err)
	var sentiments []BeanAggregate
	noerror(ds.query.Select(&sentiments, query, args...))
	return sentiments
}

const _SQL_MATCH_CLUSTERS = `
WITH 
	filtered_beans AS (
		SELECT * 
		FROM bean_embeddings 
		WHERE id IN (?)
	),
	cluster_matches AS (
		SELECT fb.id as id, mb.id as tag, array_distance(fb.embedding, mb.embedding) as distance
		FROM filtered_beans fb CROSS JOIN bean_embeddings mb
		WHERE mb.id != fb.id AND distance < %f
	)
SELECT id, tag
FROM cluster_matches cm
WHERE tag IN (
	SELECT tag FROM cluster_matches cm2
	WHERE cm2.id == cm.id 
	ORDER BY cm2.distance LIMIT %d
)
ORDER BY cm.id, cm.distance;
`

func (ds *Ducksack) MatchClusters(ids []string, threshold float64, limit int) []TagData {
	sql := fmt.Sprintf(_SQL_MATCH_CLUSTERS, threshold, limit)
	query, args, err := sqlx.In(sql, ids)
	noerror(err)
	var clusters []TagData
	noerror(ds.query.Select(&clusters, query, args...))
	return clusters
}

func (ds *Ducksack) Close() {
	noerror(ds.query.Close())
	noerror(ds.db.Close())
}

func noerror(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func logerror(err error) {
	if err != nil {
		log.Println(err)
	}
}
