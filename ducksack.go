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
	"github.com/marcboeker/go-duckdb/v2"

	// _ "github.com/mattn/go-sqlite3"
	datautils "github.com/soumitsalman/data-utils"
)

const (
	BEAN_CORES      = "bean_cores"
	BEAN_EMBEDDINGS = "bean_embeddings"
	BEAN_CLUSTERS   = "bean_clusters"
	BEAN_CATEGORIES = "bean_categories"
	BEAN_SENTIMENTS = "bean_sentiments"
	BEAN_GISTS      = "bean_gists"
	BEAN_REGIONS    = "bean_regions"
	BEAN_ENTITIES   = "bean_entities"
	BEAN_CHATTERS   = "bean_chatters"
	BEAN_AGGREGATES = "bean_aggregates"
	CHATTERS        = "chatters"
	SOURCES         = "sources"
	CATEGORIES      = "categories"
	SENTIMENTS      = "sentiments"
)

const (
	MAX_COMPUTED_TAGS = 3
	MAX_RELATED_EPS   = 0.43
)

type Ducksack struct {
	connector *duckdb.Connector
	db        *sql.DB
	query     *sqlx.DB
	dim       int
}

////////// INITIALIZE DATABASE //////////

// func NewBeanlake(datapath string, initsql string, vectordim int) *Ducksack {

// 	conn, err := duckdb.NewConnector(fmt.Sprintf("%s?threads=%d", datapath, max(1, runtime.NumCPU()-1)), nil)
// 	noerror(err)
// 	return &Ducksack{
// 		connector: conn,
// 	}
// }

func NewBeansack(datapath string, initsql string, vectordim int, cluster_eps float64) *Ducksack {
	conn, err := duckdb.NewConnector(fmt.Sprintf("%s?threads=%d", datapath, max(1, runtime.NumCPU()-1)), nil)
	noerror(err, "CONNECTOR ERROR")

	// open connection
	db := sql.OpenDB(conn)
	if initsql != "" {
		_, err = db.Exec(fmt.Sprintf(initsql, vectordim, cluster_eps))
		noerror(err, "INIT SQL ERROR")
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
	noerror(err, "CONNECTOR ERROR")
	appender, err := duckdb.NewAppenderFromConn(conn, "", table)
	noerror(err, "APPENDER ERROR")
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
		// if beans[i].Updated.IsZero() {
		// 	beans[i].Updated = now
		// }
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

// func (ds *Ducksack) StoreBeans(data []Bean) int {
// 	data = prepareBeans(data)
// 	// conn, err := ds.connector.Connect(context.Background())
// 	// noerror(err, "CONNECTOR ERROR")
// 	// appender, err := duckdb.NewAppenderFromConn(conn, "", "bean_cores")
// 	// noerror(err, "APPENDER ERROR")
// 	// defer appender.Close()
// 	sql := `INSERT INTO bean_cores
// 	(url, kind, title, title_length, content, content_length, summary, summary_length, author, source, created, collected)
// 	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`
// 	stmt, err := ds.db.Prepare(sql)
// 	noerror(err, "PREPARE ERROR")
// 	defer stmt.Close()
// 	count := 0
// 	for _, item := range data {
// 		if _, err := stmt.Exec(item.URL, item.Kind, item.Title, item.TitleLength, item.Content, item.ContentLength, item.Summary, item.SummaryLength, item.Author, item.Source, item.Created, item.Collected); err != nil {
// 			log.Println("INSERT ERROR", err)
// 		} else {
// 			count++
// 		}
// 	}
// 	return count
// }

func (ds *Ducksack) StoreBeans(beans []Bean) int {
	beans = prepareBeans(beans)
	return appendToTable(ds, BEAN_CORES, beans, func(bean Bean) []driver.Value {
		return []driver.Value{bean.URL, bean.Kind, bean.Title, bean.TitleLength, bean.Content, bean.ContentLength, bean.Summary, bean.SummaryLength, bean.Author, bean.Source, bean.Created, bean.Collected}
	})
}

func (ds *Ducksack) StoreEmbeddings(beans []Bean) int {
	return appendToTable(ds, BEAN_EMBEDDINGS, beans, func(bean Bean) []driver.Value {
		return []driver.Value{bean.URL, bean.Embedding}
	})
}

func (ds *Ducksack) RectifyExtendedFields(beans []Bean, max_computed_tags int, max_cluster_eps float32) {
	urls := datautils.Transform(beans, func(b *Bean) string {
		return b.URL
	})

	// const _SQL_INSERT_CATEGORIES = `
	// INSERT INTO bean_categories (url, category)
	// SELECT m1.url, m1.category FROM category_mappings m1
	// WHERE
	// 	m1.url IN (?) AND
	// 	m1.category IN (
	// 		SELECT category FROM category_mappings m2
	// 		WHERE m1.url == m2.url
	// 		ORDER BY m2.distance LIMIT %d
	// 	);`
	// updateItems(ds, fmt.Sprintf(_SQL_INSERT_CATEGORIES, max_computed_tags), urls)

	// const _SQL_INSERT_SENTIMENTS = `
	// INSERT INTO bean_sentiments (url, sentiment)
	// SELECT m1.url, m1.sentiment FROM sentiment_mappings m1
	// WHERE
	// 	m1.url IN (?) AND
	// 	m1.sentiment IN (
	// 		SELECT sentiment FROM sentiment_mappings m2
	// 		WHERE m1.url == m2.url
	// 		ORDER BY m2.distance LIMIT %d
	// 	);`
	// updateItems(ds, fmt.Sprintf(_SQL_INSERT_SENTIMENTS, max_computed_tags), urls)

	const _SQL_INSERT_CLUSTERS = `
	INSERT INTO bean_clusters (url, related)
	SELECT url, related FROM cluster_mappings
	WHERE url IN (?) AND distance < %f;`
	updateItems(ds, fmt.Sprintf(_SQL_INSERT_CLUSTERS, max_cluster_eps), urls)
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

////////// QUERY HELPERS //////////

func mustIn(query string, args ...any) (string, []any) {
	query, args, err := sqlx.In(query, args...)
	noerror(err, "IN ERROR")
	return query, args
}

func mustSelect[T any](ds *Ducksack, query string, args ...any) []T {
	var data []T
	noerror(ds.query.Select(&data, query, args...), "SELECT ERROR")
	return data
}

func queryItems[T any](ds *Ducksack, sql string, urls []string) []T {
	query, args := mustIn(sql, urls)
	var data []T
	noerror(ds.query.Select(&data, query, args...), "SELECT ERROR")
	return data
}

func updateItems(ds *Ducksack, expr string, urls []string) {
	query, args := mustIn(expr, urls)
	_, err := ds.db.Exec(query, args...)
	noerror(err, "UPDATE ERROR")
}

////////// DIRECT QUERY/GET FUNCTIONS //////////

func (ds *Ducksack) Exists(urls []string) []string {
	return queryItems[string](ds, "SELECT url FROM bean_cores WHERE url IN (?);", urls)
}

func (ds *Ducksack) GetBeans(urls []string) []Bean {
	const _SQL_QUERY_BEANS = `
	SELECT * FROM bean_aggregates
	WHERE url IN (?);`
	return queryItems[Bean](ds, _SQL_QUERY_BEANS, urls)
}

func (ds *Ducksack) GetEmbeddings(urls []string) []Bean {
	return queryItems[Bean](ds, "SELECT * FROM bean_embeddings WHERE url IN (?);", urls)
}

func (ds *Ducksack) GetGists(urls []string) []Bean {
	return queryItems[Bean](ds, "SELECT * FROM bean_gists WHERE url IN (?);", urls)
}

func (ds *Ducksack) GetRegions(urls []string) []Bean {
	const _SQL_QUERY_REGIONS = `
	SELECT url, regions FROM bean_aggregates
	WHERE url IN (?);`
	return queryItems[Bean](ds, _SQL_QUERY_REGIONS, urls)
}

func (ds *Ducksack) GetEntities(urls []string) []Bean {
	const _SQL_QUERY_ENTITIES = `
	SELECT url, entities FROM bean_aggregates
	WHERE url IN (?);`
	return queryItems[Bean](ds, _SQL_QUERY_ENTITIES, urls)
}

func (ds *Ducksack) GetCategories(urls []string) []Bean {
	const _SQL_QUERY_CATEGORIES = `
	SELECT url, categories FROM bean_aggregates
	WHERE url IN (?);`
	return queryItems[Bean](ds, _SQL_QUERY_CATEGORIES, urls)
}

func (ds *Ducksack) GetSentiments(urls []string) []Bean {
	const _SQL_QUERY_SENTIMENTS = `
	SELECT url, sentiments FROM bean_aggregates
	WHERE url IN (?);`
	return queryItems[Bean](ds, _SQL_QUERY_SENTIMENTS, urls)
}

func (ds *Ducksack) GetRelated(urls []string) []Bean {
	const _SQL_QUERY_CLUSTERS = `
	SELECT url, LIST(DISTINCT related) AS related FROM bean_clusters
	WHERE url IN (?) 
	GROUP BY url;`
	return queryItems[Bean](ds, _SQL_QUERY_CLUSTERS, urls)
}

func (ds *Ducksack) GetChatters(urls []string) []Chatter {
	query, args := mustIn("SELECT * FROM chatters WHERE bean_url IN (?) ORDER BY collected DESC;", urls)
	return mustSelect[Chatter](ds, query, args...)
}

func (ds *Ducksack) GetBeanChatters(urls []string) []ChatterAggregate {
	query, args := mustIn("SELECT * FROM bean_chatters WHERE url IN (?);", urls)
	return mustSelect[ChatterAggregate](ds, query, args...)
}

////////// DISTINCT ITEMS //////////

func (ds *Ducksack) DistinctRegions() []string {
	const _SQL_GET_ALL_REGIONS = `SELECT DISTINCT region FROM bean_regions;`
	return mustSelect[string](ds, _SQL_GET_ALL_REGIONS)
}

func (ds *Ducksack) DistinctEntities() []string {
	const _SQL_GET_ALL_ENTITIES = `SELECT DISTINCT entity FROM bean_entities;`
	return mustSelect[string](ds, _SQL_GET_ALL_ENTITIES)
}

func (ds *Ducksack) DistinctCategories() []string {
	const _SQL_GET_ALL_CATEGORIES = `SELECT DISTINCT category FROM bean_categories;`
	return mustSelect[string](ds, _SQL_GET_ALL_CATEGORIES)
}

func (ds *Ducksack) DistinctSentiments() []string {
	const _SQL_GET_ALL_SENTIMENTS = `SELECT DISTINCT sentiment FROM bean_sentiments;`
	return mustSelect[string](ds, _SQL_GET_ALL_SENTIMENTS)
}

func (ds *Ducksack) DistinctSources() []string {
	const _SQL_GET_ALL_SOURCES = `SELECT base_url AS value FROM sources;`
	return mustSelect[string](ds, _SQL_GET_ALL_SOURCES)
}

//////////// COMPOSITE QUERIES ///////////

const _SQL_QUERY_BEAN_AGGREGATES = `SELECT * FROM bean_aggregates`

func (ds *Ducksack) QueryBeans(
	kind string,
	created_after time.Time,
	categories []string,
	regions []string,
	entities []string,
	sources []string,
	offset int64, limit int64) []Bean {
	sql, where_params := addWhere(_SQL_QUERY_BEAN_AGGREGATES, kind, created_after, categories, regions, entities, sources, 0)
	sql, paging_params := addPagination(sql, offset, limit)
	return mustSelect[Bean](ds, sql, append(where_params, paging_params...)...)
}

func (ds *Ducksack) VectorSearchBeans(
	embedding []float32, max_distance float64,
	kind string,
	created_after time.Time,
	categories []string,
	regions []string,
	entities []string,
	sources []string,
	offset int64, limit int64) []Bean {

	filtered_sql, params := addWhere(_SQL_QUERY_BEAN_AGGREGATES, kind, created_after, categories, regions, entities, sources, 0)
	distances_sql := `
	SELECT *, array_cosine_distance(embedding, ?::FLOAT[%d]) AS distance 
	FROM filtered`
	distances_sql, distances_where_params := addWhere(fmt.Sprintf(distances_sql, ds.dim), "", time.Time{}, nil, nil, nil, nil, max_distance)
	distances_sql = addOrderBy(distances_sql, "distance DESC")
	distances_sql, distances_pagination_params := addPagination(distances_sql, offset, limit)

	params = append(params, Float32Array(embedding))
	params = append(params, distances_where_params...)
	params = append(params, distances_pagination_params...)

	sql := `
	WITH filtered AS ( %s ), distances AS ( %s )
	SELECT * EXCLUDE(distance) FROM distances;`
	return mustSelect[Bean](ds, fmt.Sprintf(sql, filtered_sql, distances_sql), params...)
}

func addWhere(
	base_sql string,
	kind string,
	created_after time.Time,
	categories []string,
	regions []string,
	entities []string,
	sources []string,
	max_distance float64) (string, []any) {
	params := []any{}
	where_exprs := []string{}

	if len(kind) > 0 {
		where_exprs = append(where_exprs, "kind = ?")
		params = append(params, kind)
	}
	if !created_after.IsZero() {
		where_exprs = append(where_exprs, "created >= ?")
		params = append(params, created_after)
	}
	if len(categories) > 0 {
		where_exprs = append(where_exprs, "ARRAY_HAS_ANY(categories, ?)")
		params = append(params, StringArray(categories))
	}
	if len(regions) > 0 {
		where_exprs = append(where_exprs, "ARRAY_HAS_ANY(regions, ?)")
		params = append(params, StringArray(regions))
	}
	if len(entities) > 0 {
		where_exprs = append(where_exprs, "ARRAY_HAS_ANY(entities, ?)")
		params = append(params, StringArray(entities))
	}
	if len(sources) > 0 {
		where_exprs = append(where_exprs, "source IN (?)")
		params = append(params, sources)
	}
	if max_distance > 0 {
		where_exprs = append(where_exprs, "distance <= ?")
		params = append(params, max_distance)
	}

	if len(where_exprs) > 0 {
		base_sql = fmt.Sprintf("%s WHERE %s", base_sql, strings.Join(where_exprs, " AND "))
	}
	return mustIn(base_sql, params...)
}

func addPagination(base_sql string, offset int64, limit int64) (string, []any) {
	params := []any{}
	if offset > 0 {
		base_sql = fmt.Sprintf("%s OFFSET ?", base_sql)
		params = append(params, offset)
	}
	if limit > 0 {
		base_sql = fmt.Sprintf("%s LIMIT ?", base_sql)
		params = append(params, limit)
	}
	return base_sql, params
}

func addOrderBy(base_sql string, fields ...string) string {
	return fmt.Sprintf("%s ORDER BY %s", base_sql, strings.Join(fields, ", "))
}

func (ds *Ducksack) Close() {
	noerror(ds.query.Close(), "QUERY CLOSE ERROR")
	noerror(ds.db.Close(), "DB CLOSE ERROR")
}

func noerror(err error, msg string) {
	if err != nil {
		log.Fatal(msg, ": ", err)
	}
}

func logerror(err error) {
	if err != nil {
		log.Println(err)
	}
}
