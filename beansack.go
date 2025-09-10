package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jmoiron/sqlx"
	"github.com/marcboeker/go-duckdb/v2"
	datautils "github.com/soumitsalman/data-utils"
	// _ "github.com/mattn/go-sqlite3"
)

const (
	BEAN_CORES       = "bean_cores"
	BEAN_EMBEDDINGS  = "bean_embeddings"
	BEAN_CLUSTERS    = "bean_clusters"
	BEAN_CATEGORIES  = "bean_categories"
	BEAN_SENTIMENTS  = "bean_sentiments"
	BEAN_GISTS       = "bean_gists"
	BEAN_REGIONS     = "bean_regions"
	BEAN_ENTITIES    = "bean_entities"
	BEAN_CHATTERS    = "bean_chatters"
	AGGREGATES_BEANS = "aggregated_beans"
	UNTAGGED_BEANS   = "untagged_beans_view"
	CHATTERS         = "chatters"
	SOURCES          = "sources"
	CATEGORIES       = "categories"
	SENTIMENTS       = "sentiments"
)

const (
	MAX_COMPUTED_TAGS = 3
	MAX_RELATED_EPS   = 0.43
)

const (
	HAS_CHATTERS      = "shares != 0"
	GIST_IS_NOT_NULL  = "gist IS NOT NULL"
	ORDER_BY_DISTANCE = "distance ASC"
	ORDER_BY_CREATED  = "created DESC"
	ORDER_BY_UPDATED  = "DATE(updated) DESC, comments DESC, likes DESC, shares DESC"
)

type BeanSack struct {
	connector     *duckdb.Connector
	db            *sql.DB
	query         *sqlx.DB
	dim           int
	needs_refresh atomic.Bool
}

////////// STORING FUNCTIONS //////////

func (ds *BeanSack) StoreBeans(beans []Bean) int {
	beans = prepareBeans(beans)
	count := appendToTable(ds, BEAN_CORES, beans, func(bean Bean) []driver.Value {
		return []driver.Value{bean.URL, bean.Kind, bean.Title, bean.TitleLength, bean.Content, bean.ContentLength, bean.RestrictedContent, bean.Summary, bean.SummaryLength, bean.Author, bean.Source, bean.Created, bean.Collected}
	})
	ds.needs_refresh.Store(true)
	return count
}

func (ds *BeanSack) StoreEmbeddings(beans []Bean) int {
	beans = datautils.Filter(beans, func(bean *Bean) bool {
		return len(bean.Embedding) == ds.dim
	})
	count := appendToTable(ds, BEAN_EMBEDDINGS, beans, func(bean Bean) []driver.Value {
		return []driver.Value{bean.URL, bean.Embedding}
	})
	ds.needs_refresh.Store(true)
	return count
}

func (ds *BeanSack) StoreTags(beans []Bean) int {
	tags := prepareTags(beans)
	count := 0
	if categories, ok := tags[BEAN_CATEGORIES]; ok {
		count += ds.storeTagsToTable(categories, BEAN_CATEGORIES)
	}
	if sentiments, ok := tags[BEAN_SENTIMENTS]; ok {
		count += ds.storeTagsToTable(sentiments, BEAN_SENTIMENTS)
	}
	if regions, ok := tags[BEAN_REGIONS]; ok {
		count += ds.storeTagsToTable(regions, BEAN_REGIONS)
	}
	if entities, ok := tags[BEAN_ENTITIES]; ok {
		count += ds.storeTagsToTable(entities, BEAN_ENTITIES)
	}
	if gist, ok := tags[BEAN_GISTS]; ok {
		count += ds.storeTagsToTable(gist, BEAN_GISTS)
	}
	return count
}

func (ds *BeanSack) StoreChatters(chatters []Chatter) int {
	chatters = prepareChatters(chatters)
	count := appendToTable(ds, CHATTERS, chatters, func(chatter Chatter) []driver.Value {
		return []driver.Value{chatter.ChatterURL, chatter.BeanURL, chatter.Collected, chatter.Source, chatter.Forum, chatter.Likes, chatter.Comments, chatter.Subscribers}
	})
	ds.needs_refresh.Store(true)
	return count
}

func (ds *BeanSack) StoreSources(sources []Source) int {
	return appendToTable(ds, SOURCES, sources, func(source Source) []driver.Value {
		return []driver.Value{source.Name, source.Description, source.BaseURL, source.DomainName, source.Favicon, source.RSSFeed}
	})
}

func appendToTable[T any](ds *BeanSack, table string, data []T, getfieldvalues func(item T) []driver.Value) int {
	if data == nil {
		return 0
	}
	conn, err := ds.connector.Connect(context.Background())
	if err != nil {
		logerrorf(err, "append failed: table=%s num_items=%d", table, len(data))
		return -1
	}

	appender, err := duckdb.NewAppenderFromConn(conn, "", table)
	if err != nil {
		logerrorf(err, "append failed: table=%s num_items=%d", table, len(data))
		return -1
	}
	defer appender.Close()

	count := 0
	errs := []error{}
	for _, item := range data {
		if err := appender.AppendRow(getfieldvalues(item)...); err != nil {
			errs = append(errs, err)
		} else {
			count++
		}
	}
	logwarningf(errors.Join(errs...), "append had errors: table=%s", table)
	log.WithFields(log.Fields{"table": table, "num_items": count}).Info("append succeeded")
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

func flattenTags(url string, data []string) []TagData {
	return datautils.FilterAndTransform(data, func(tag *string) (bool, TagData) {
		return len(*tag) > 0, TagData{URL: url, Tag: *tag}
	})
}

func prepareTags(beans []Bean) map[string][]TagData {
	// rough initialization
	results := map[string][]TagData{
		BEAN_CATEGORIES: make([]TagData, 0, 3*len(beans)),
		BEAN_SENTIMENTS: make([]TagData, 0, 3*len(beans)),
		BEAN_REGIONS:    make([]TagData, 0, 3*len(beans)),
		BEAN_ENTITIES:   make([]TagData, 0, 3*len(beans)),
		BEAN_GISTS:      make([]TagData, 0, len(beans)),
	}
	for _, bean := range beans {
		if len(bean.Categories) > 0 {
			results[BEAN_CATEGORIES] = append(results[BEAN_CATEGORIES], flattenTags(bean.URL, bean.Categories)...)
		}
		if len(bean.Sentiments) > 0 {
			results[BEAN_SENTIMENTS] = append(results[BEAN_SENTIMENTS], flattenTags(bean.URL, bean.Sentiments)...)
		}
		if len(bean.Regions) > 0 {
			results[BEAN_REGIONS] = append(results[BEAN_REGIONS], flattenTags(bean.URL, bean.Regions)...)
		}
		if len(bean.Entities) > 0 {
			results[BEAN_ENTITIES] = append(results[BEAN_ENTITIES], flattenTags(bean.URL, bean.Entities)...)
		}
		if len(bean.Gist) > 0 {
			results[BEAN_GISTS] = append(results[BEAN_GISTS], TagData{URL: bean.URL, Tag: bean.Gist})
		}
	}
	return results
}

func (ds *BeanSack) storeTagsToTable(tags []TagData, tag_table string) int {
	if len(tags) == 0 {
		return 0
	}
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

const _REFRESH_AGGREGATES = `
TRUNCATE TABLE aggregated_beans;
INSERT INTO aggregated_beans SELECT * FROM aggregated_beans_view;
`

func (ds *BeanSack) Refresh() {
	log.Info("refresh triggered")
	if ds.needs_refresh.Load() {
		_, err := ds.db.Exec(_REFRESH_AGGREGATES)
		if err != nil {
			logwarningf(err, "refresh aggregates failed")
		} else {
			log.Info("refresh aggregates succeeded")
		}
		ds.needs_refresh.Store(false)
	}
}

////////// QUERY HELPERS //////////

func mustIn(query string, args ...any) (string, []any) {
	query, args, err := sqlx.In(query, args...)
	noerror(err, "IN ERROR")
	return query, args
}

func mustSelect[T any](ds *BeanSack, query string, args ...any) []T {
	var data []T
	noerror(ds.query.Select(&data, query, args...), "query failed")
	log.WithFields(log.Fields{"query": query, "params": len(args), "num_items": len(data)}).Info("query succeeded")
	return data
}

func shouldIn(query string, args ...any) (string, []any, error) {
	query, args, err := sqlx.In(query, args...)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{"query": query, "params": len(args)}).Error("query failed")
	}
	return query, args, err
}

func shouldSelect[T any](ds *BeanSack, query string, args ...any) ([]T, error) {
	var data []T
	err := ds.query.Select(&data, query, args...)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{"query": query, "params": len(args)}).Error("query failed")
	} else {
		log.WithFields(log.Fields{"query": query, "params": len(args), "num_items": len(data)}).Info("query succeeded")
	}
	return data, err
}

func getBeans[T any](ds *BeanSack, query string, urls []string) []T {
	if len(urls) == 0 {
		return nil
	}
	query, args, err := shouldIn(query, urls)
	if err != nil {
		return nil
	}
	beans, err := shouldSelect[T](ds, query, args...)
	if err != nil {
		return nil
	}
	return beans
}

////////// DIRECT QUERY/GET FUNCTIONS //////////

func (ds *BeanSack) Exists(urls []string) []string {
	return getBeans[string](ds, "SELECT url FROM bean_cores WHERE url IN (?);", urls)
}

func (ds *BeanSack) GetBeans(urls []string) []Bean {
	const _SQL_QUERY_BEANS = `
	SELECT * FROM bean_aggregates
	WHERE url IN (?);`
	return getBeans[Bean](ds, _SQL_QUERY_BEANS, urls)
}

func (ds *BeanSack) GetEmbeddings(urls []string) []Bean {
	return getBeans[Bean](ds, "SELECT * FROM bean_embeddings WHERE url IN (?);", urls)
}

func (ds *BeanSack) GetGists(urls []string) []Bean {
	return getBeans[Bean](ds, "SELECT * FROM bean_gists WHERE url IN (?);", urls)
}

func (ds *BeanSack) GetRegions(urls []string) []Bean {
	const _SQL_QUERY_REGIONS = `
	SELECT url, regions FROM bean_aggregates
	WHERE url IN (?);`
	return getBeans[Bean](ds, _SQL_QUERY_REGIONS, urls)
}

func (ds *BeanSack) GetEntities(urls []string) []Bean {
	const _SQL_QUERY_ENTITIES = `
	SELECT url, entities FROM bean_aggregates
	WHERE url IN (?);`
	return getBeans[Bean](ds, _SQL_QUERY_ENTITIES, urls)
}

func (ds *BeanSack) GetCategories(urls []string) []Bean {
	const _SQL_QUERY_CATEGORIES = `
	SELECT url, categories FROM bean_aggregates
	WHERE url IN (?);`
	return getBeans[Bean](ds, _SQL_QUERY_CATEGORIES, urls)
}

func (ds *BeanSack) GetSentiments(urls []string) []Bean {
	const _SQL_QUERY_SENTIMENTS = `
	SELECT url, sentiments FROM bean_aggregates
	WHERE url IN (?);`
	return getBeans[Bean](ds, _SQL_QUERY_SENTIMENTS, urls)
}

func (ds *BeanSack) GetRelated(urls []string) []Bean {
	const _SQL_QUERY_CLUSTERS = `
	SELECT url, LIST(DISTINCT related) AS related FROM bean_clusters
	WHERE url IN (?) 
	GROUP BY url;`
	return getBeans[Bean](ds, _SQL_QUERY_CLUSTERS, urls)
}

func (ds *BeanSack) GetChatters(urls []string) []Chatter {
	return getBeans[Chatter](ds, "SELECT * FROM chatters WHERE bean_url IN (?) ORDER BY collected DESC;", urls)
}

func (ds *BeanSack) GetBeanChatters(urls []string) []ChatterAggregate {
	return getBeans[ChatterAggregate](ds, "SELECT * FROM bean_chatters WHERE url IN (?);", urls)
}

func (ds *BeanSack) GetSources(domain_names []string) []Source {
	const _SQL_QUERY_SOURCES = `
	SELECT * FROM sources
	WHERE domain_name IN (?);`
	return getBeans[Source](ds, _SQL_QUERY_SOURCES, domain_names)
}

////////// DISTINCT ITEMS //////////

func (ds *BeanSack) DistinctRegions() []string {
	const _SQL_GET_ALL_REGIONS = `SELECT DISTINCT region FROM bean_regions;`
	return mustSelect[string](ds, _SQL_GET_ALL_REGIONS)
}

func (ds *BeanSack) DistinctEntities() []string {
	const _SQL_GET_ALL_ENTITIES = `SELECT DISTINCT entity FROM bean_entities;`
	return mustSelect[string](ds, _SQL_GET_ALL_ENTITIES)
}

func (ds *BeanSack) DistinctCategories() []string {
	const _SQL_GET_ALL_CATEGORIES = `SELECT DISTINCT category FROM bean_categories;`
	return mustSelect[string](ds, _SQL_GET_ALL_CATEGORIES)
}

func (ds *BeanSack) DistinctSentiments() []string {
	const _SQL_GET_ALL_SENTIMENTS = `SELECT DISTINCT sentiment FROM bean_sentiments;`
	return mustSelect[string](ds, _SQL_GET_ALL_SENTIMENTS)
}

func (ds *BeanSack) DistinctSources() []string {
	const _SQL_GET_ALL_SOURCES = `SELECT DISTINCT source FROM bean_cores;`
	return mustSelect[string](ds, _SQL_GET_ALL_SOURCES)
}

// ////////// COMPOSITE QUERIES ///////////

// func (ds *Ducksack) QueryBeanAggregates(
// 	urls []string,
// 	kind string,
// 	created_after time.Time,
// 	categories []string,
// 	regions []string,
// 	entities []string,
// 	sources []string,
// 	embedding []float32,
// 	max_distance float64,
// 	order []string,
// 	offset int,
// 	limit int,
// 	columns []string,
// ) []Bean {
// 	query := NewSelect(ds).
// 		Table(BEAN_AGGREGATES).
// 		Columns(columns...).
// 		WhereForCustomColumns(
// 			urls,
// 			kind,
// 			created_after,
// 			categories,
// 			regions,
// 			entities,
// 			sources,
// 			embedding,
// 			max_distance,
// 		).
// 		Order(order...).
// 		Limit(limit).
// 		Offset(offset)

// 	return ds.QueryBeans(query)
// }

// const _SQL_QUERY_BEANS_SELECT_FIELDS = `
// SELECT %s FROM bean_aggregates
// %s
// %s
// %s;`

// func (ds *Ducksack) QueryBeansWithSelectFields(
// 	kind string,
// 	created_after time.Time,
// 	categories []string,
// 	regions []string,
// 	entities []string,
// 	sources []string,
// 	addtional_where []string,
// 	order_by []string,
// 	offset int64, limit int64,
// 	select_fields []string) ([]Bean, error) {

// 	select_fields_sql := createSelectFields(select_fields...)
// 	where_exprs, where_params := CreateWhereExprsForFieldValues(kind, created_after, categories, regions, entities, sources, 0)
// 	where_sql := CombineWhereExprs(append(where_exprs, addtional_where...)...)
// 	order_by_sql := CreateOrderByExprs(order_by...)
// 	paging_sql, paging_params := CreatePaginationExprs(offset, limit)

// 	sql := fmt.Sprintf(_SQL_QUERY_BEANS_SELECT_FIELDS, select_fields_sql, where_sql, order_by_sql, paging_sql)
// 	sql, params := mustIn(sql, append(where_params, paging_params...)...)
// 	return shouldSelect[Bean](ds, sql, params...)
// }

// func (ds *Ducksack) VectorSearchBeansWithSelectFields(
// 	embedding []float32, max_distance float64,
// 	kind string,
// 	created_after time.Time,
// 	categories []string,
// 	regions []string,
// 	entities []string,
// 	sources []string,
// 	addtional_where []string,
// 	order_by []string,
// 	offset int64, limit int64,
// 	select_fields []string) ([]Bean, error) {

// 	if len(embedding) == 0 {
// 		return nil, errors.New("embedding is required")
// 	}

// 	select_fields_sql := createSelectFields(select_fields...)
// 	select_fields_sql = fmt.Sprintf("%s, array_cosine_distance(embedding, ?::FLOAT[%d]) AS distance", select_fields_sql, ds.dim)
// 	where_exprs, where_params := CreateWhereExprsForFieldValues(kind, created_after, categories, regions, entities, sources, max_distance)
// 	where_sql := CombineWhereExprs(append(where_exprs, addtional_where...)...)
// 	order_by_sql := CreateOrderByExprs(order_by...)
// 	paging_sql, paging_params := CreatePaginationExprs(offset, limit)

// 	sql := fmt.Sprintf(_SQL_QUERY_BEANS_SELECT_FIELDS, select_fields_sql, where_sql, order_by_sql, paging_sql)
// 	params := []any{Float32Array(embedding)}
// 	params = append(params, where_params...)
// 	params = append(params, paging_params...)
// 	sql, params = mustIn(sql, params...)
// 	pp.Println("sql", sql, params)
// 	return shouldSelect[Bean](ds, sql, params...)
// }

// const _SQL_QUERY_BEAN_CORES = `
// SELECT * FROM bean_cores
// %s
// %s
// %s;`

// const _SQL_QUERY_BEAN_AGGREGATES = `
// SELECT * FROM bean_aggregates
// %s
// %s
// %s;`

// func (ds *Ducksack) QueryBeanAggregates(where []string, order_by []string, offset int64, limit int64) ([]Bean, error) {
// 	where_sql := CombineWhereExprs(where...)
// 	order_by_sql := CreateOrderByExprs(order_by...)
// 	paging_sql, paging_params := CreatePaginationExprs(offset, limit)
// 	sql := fmt.Sprintf(_SQL_QUERY_BEAN_AGGREGATES, where_sql, order_by_sql, paging_sql)
// 	return shouldSelect[Bean](ds, sql, paging_params...)
// }

func (ds *BeanSack) QueryBeanCores(
	where []string,
	order []string,
	offset int,
	limit int,
) []Bean {
	query := NewSelect(ds).
		Table(BEAN_CORES).
		Where(where...).
		Offset(offset).
		Limit(limit)

	return ds.QueryBeans(query)
}

func (ds *BeanSack) QueryAggregatedBeans(
	where []string,
	order []string,
	offset int,
	limit int,
) []Bean {
	query := NewSelect(ds).
		Table(AGGREGATES_BEANS).
		Where(where...).
		Offset(offset).
		Limit(limit)

	return ds.QueryBeans(query)
}

func (ds *BeanSack) QueryBeans(query *SelectExpr) []Bean {
	sql, params := query.ToSQL()
	sql, params, err := shouldIn(sql, params...)
	if err != nil {
		return nil
	}
	beans, err := shouldSelect[Bean](ds, sql, params...)
	if err != nil {
		return nil
	}
	return beans
}

const _SQL_DELETE_BEAN_CORES = `DELETE FROM bean_cores %s;`
const _SQL_DELETE_BEAN_EMBEDDINGS = `DELETE FROM bean_embeddings WHERE url IN (SELECT url FROM bean_cores %s);`
const _SQL_DELETE_BEAN_GISTS = `DELETE FROM bean_gists WHERE url IN (SELECT url FROM bean_cores %s);`
const _SQL_DELETE_BEAN_CATEGORIES = `DELETE FROM bean_categories WHERE url IN (SELECT url FROM bean_cores %s);`
const _SQL_DELETE_BEAN_SENTIMENTS = `DELETE FROM bean_sentiments WHERE url IN (SELECT url FROM bean_cores %s);`
const _SQL_DELETE_BEAN_REGIONS = `DELETE FROM bean_regions WHERE url IN (SELECT url FROM bean_cores %s);`
const _SQL_DELETE_BEAN_ENTITIES = `DELETE FROM bean_entities WHERE url IN (SELECT url FROM bean_cores %s);`
const _SQL_DELETE_BEAN_CHATTERS = `DELETE FROM chatters WHERE bean_url IN (SELECT url FROM bean_cores %s);`
const _SQL_DELETE_CHATTERS = `DELETE FROM chatters %s;`
const _SQL_DELETE_SOURCES = `DELETE FROM sources %s;`

func (ds *BeanSack) DeleteBeans(wheres ...string) error {
	where_sql := combineWhereExprs(wheres...)
	errs := []error{}
	_, err := ds.db.Exec(fmt.Sprintf(_SQL_DELETE_BEAN_GISTS, where_sql))
	errs = append(errs, err)
	_, err = ds.db.Exec(fmt.Sprintf(_SQL_DELETE_BEAN_EMBEDDINGS, where_sql))
	errs = append(errs, err)
	_, err = ds.db.Exec(fmt.Sprintf(_SQL_DELETE_BEAN_CATEGORIES, where_sql))
	errs = append(errs, err)
	_, err = ds.db.Exec(fmt.Sprintf(_SQL_DELETE_BEAN_SENTIMENTS, where_sql))
	errs = append(errs, err)
	_, err = ds.db.Exec(fmt.Sprintf(_SQL_DELETE_BEAN_REGIONS, where_sql))
	errs = append(errs, err)
	_, err = ds.db.Exec(fmt.Sprintf(_SQL_DELETE_BEAN_ENTITIES, where_sql))
	errs = append(errs, err)
	_, err = ds.db.Exec(fmt.Sprintf(_SQL_DELETE_BEAN_CHATTERS, where_sql))
	errs = append(errs, err)
	_, err = ds.db.Exec(fmt.Sprintf(_SQL_DELETE_BEAN_CORES, where_sql))
	errs = append(errs, err)
	return errors.Join(errs...)
}

func (ds *BeanSack) DeleteChatters(wheres ...string) error {
	where_sql := combineWhereExprs(wheres...)
	_, err := ds.db.Exec(fmt.Sprintf(_SQL_DELETE_CHATTERS, where_sql))
	noerror(err, "DELETE CHATTERS ERROR")
	return err
}

func (ds *BeanSack) DeleteSources(wheres ...string) error {
	where_sql := combineWhereExprs(wheres...)
	_, err := ds.db.Exec(fmt.Sprintf(_SQL_DELETE_SOURCES, where_sql))
	noerror(err, "DELETE SOURCES ERROR")
	return err
}

func combineWhereExprs(exprs ...string) string {
	if len(exprs) > 0 {
		return fmt.Sprintf("WHERE %s", strings.Join(exprs, " AND "))
	}
	return ""
}

// func createSelectFields(field_exprs ...string) string {
// 	if len(field_exprs) > 0 {
// 		return strings.Join(field_exprs, ", ")
// 	}
// 	return "*"
// }

// func CreateWhereExprsForFieldValues(
// 	kind string,
// 	created_after time.Time,
// 	categories []string,
// 	regions []string,
// 	entities []string,
// 	sources []string,
// 	max_distance float64) ([]string, []any) {

// 	params := []any{}
// 	where_exprs := []string{}

// 	if len(kind) > 0 {
// 		where_exprs = append(where_exprs, "kind = ?")
// 		params = append(params, kind)
// 	}
// 	if !created_after.IsZero() {
// 		where_exprs = append(where_exprs, "created >= ?")
// 		params = append(params, created_after)
// 	}
// 	if len(categories) > 0 {
// 		where_exprs = append(where_exprs, "ARRAY_HAS_ANY(categories, ?)")
// 		params = append(params, StringArray(categories))
// 	}
// 	if len(regions) > 0 {
// 		where_exprs = append(where_exprs, "ARRAY_HAS_ANY(regions, ?)")
// 		params = append(params, StringArray(regions))
// 	}
// 	if len(entities) > 0 {
// 		where_exprs = append(where_exprs, "ARRAY_HAS_ANY(entities, ?)")
// 		params = append(params, StringArray(entities))
// 	}
// 	if len(sources) > 0 {
// 		where_exprs = append(where_exprs, "source IN (?)")
// 		params = append(params, sources)
// 	}
// 	if max_distance > 0 {
// 		where_exprs = append(where_exprs, "distance <= ?")
// 		params = append(params, max_distance)
// 	}
// 	return where_exprs, params
// }

// func CreateWhereExprsForMissingTags(tagnames []string) []string {
// 	exprs := make([]string, 0, len(tagnames))
// 	for _, tag := range tagnames {
// 		table_name := ""
// 		switch tag {
// 		case "gist":
// 			table_name = "bean_gists"
// 		case "embedding":
// 			table_name = "bean_embeddings"
// 		case "category":
// 			table_name = "bean_categories"
// 		case "sentiment":
// 			table_name = "bean_sentiments"
// 		case "region":
// 		case "regions":
// 			table_name = "bean_regions"
// 		case "entity":
// 		case "entities":
// 			table_name = "bean_entities"
// 		}
// 		if table_name != "" {
// 			exprs = append(exprs, fmt.Sprintf("url NOT IN (SELECT url FROM %s)", table_name))
// 		}
// 	}
// 	return exprs
// }

// func CreatePaginationExprs(offset int64, limit int64) (string, []any) {
// 	exprs := []string{}
// 	params := []any{}
// 	if offset > 0 {
// 		exprs = append(exprs, "OFFSET ?")
// 		params = append(params, offset)
// 	}
// 	if limit > 0 {
// 		exprs = append(exprs, "LIMIT ?")
// 		params = append(params, limit)
// 	}
// 	return strings.Join(exprs, " "), params
// }

// func CreateOrderByExprs(fields ...string) string {
// 	if len(fields) > 0 {
// 		return fmt.Sprintf("ORDER BY %s", strings.Join(fields, ", "))
// 	}
// 	return ""
// }

////////// ADMIN COMMANDS ///////////

func (ds *BeanSack) Execute(commands ...string) error {
	errs := []error{}
	for _, command := range commands {
		_, err := ds.db.Exec(command)
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (ds *BeanSack) Close() {
	noerror(ds.query.Close(), "QUERY CLOSE ERROR")
	noerror(ds.db.Close(), "DB CLOSE ERROR")
}
