package beansack

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/marcboeker/go-duckdb/v2"
)

const (
	BEAN_CORES      = "bean_cores"
	BEAN_EMBEDDINGS = "bean_embeddings"
	BEAN_CLUSTERS   = "computed_bean_clusters"
	BEAN_CATEGORIES = "computed_bean_categories"
	BEAN_SENTIMENTS = "computed_bean_sentiments"
	BEAN_GISTS      = "bean_gists"
	PUBLISHERS      = "publishers"
	CHATTERS        = "chatters"
	PROCESSED_BEANS = "processed_beans_view"
	LATEST_BEANS    = "latest_beans_view"
	TRENDING_BEANS  = "trending_beans_view"
	BEAN_CHATTERS   = "bean_chatters_view"
)

const (
	DIGEST_COLUMNS = "url, created, gist, categories, sentiments"
	PUBLIC_COLUMNS = `* EXCLUDE(gist, embedding)`
)

const (
	ORDER_BY_DISTANCE = "distance ASC"
	ORDER_BY_TRENDING = "DATE(updated) DESC, comments DESC, likes DESC, shares DESC"
	ORDER_BY_LATEST   = "created DESC"
)

const SQL_INIT = `
INSTALL ducklake;
LOAD ducklake;

INSTALL postgres;
LOAD postgres;

ATTACH 'ducklake:%s' AS warehouse 
(
    METADATA_SCHEMA 'beansack',
    DATA_PATH '%s'
);
USE warehouse;

DROP VIEW IF EXISTS latest_beans_view;
CREATE VIEW latest_beans_view AS
SELECT  
	url,
	kind,
	title,
	COALESCE(summary, '') AS summary,
	COALESCE(author, '') AS author,
	source,
	COALESCE(image_url, '') AS image_url,
	created,
	embedding,
	gist,
	categories,
	sentiments,
	regions,
	entities,
	cluster_id,
	cluster_size
FROM processed_beans_view;

DROP VIEW IF EXISTS trending_beans_view;
CREATE VIEW trending_beans_view AS
SELECT * EXCLUDE(ch.url, ch.collected), ch.collected as updated
FROM latest_beans_view b
INNER JOIN bean_chatters_view ch ON b.url = ch.url;
`

type Beansack struct {
	db  *sql.DB
	dbx *sqlx.DB
}

// NewBeansack creates a new Beansack connection and executes initialization SQL.
func NewReadonlyBeansack(catalogdb, storagedb string) (*Beansack, error) {
	if strings.HasPrefix(catalogdb, "postgresql://") {
		catalogdb = "postgres:" + catalogdb
	}

	init_sql := fmt.Sprintf(SQL_INIT, catalogdb, storagedb)

	// Open DuckDB via database/sql. The driver registers itself with the name "duckdb".
	db, err := sql.Open("duckdb", "")
	if err != nil {
		LogError(err, "DB driver failed")
		return nil, err
	}
	// Execute init SQL. Use Exec which can run multiple statements when supported by driver.
	if _, err := db.Exec(init_sql); err != nil {
		db.Close()
		LogError(err, "DB connection failed")
		return nil, err
	}

	log.Printf("DB initialized")
	return &Beansack{
		db:  db,
		dbx: sqlx.NewDb(db, "duckdb"),
	}, nil
}

// query syntax builder

func selectExpr(table string, columns []string, embedding []float32) (string, []any) {
	cols := columns
	if len(cols) == 0 {
		cols = []string{"*"}
	}
	if len(embedding) > 0 {
		cols = append(cols, fmt.Sprintf("array_cosine_distance(embedding::FLOAT[%d], ?::FLOAT[%d]) AS distance", len(embedding), len(embedding)))
	}
	expr := fmt.Sprintf("SELECT %s FROM warehouse.%s", strings.Join(cols, ", "), table)
	params := []any{}
	if len(embedding) > 0 {
		params = append(params, Float32Array(embedding))
	}
	return expr, params
}

func whereExpr(
	urls []string,
	kinds []string,
	authors, sources []string,
	created time.Time, collected time.Time, updated time.Time,
	categories, regions, entities []string,
	embedding []float32, distance float64,
	exprs []string,
) (string, []any) {
	var conds []string
	var params []any
	if len(urls) == 1 {
		conds = append(conds, "url = ?")
		params = append(params, urls[0])
	} else if len(urls) > 1 {
		conds = append(conds, "url IN (?)")
		params = append(params, urls)
	}

	if len(kinds) == 1 {
		conds = append(conds, "kind = ?")
		params = append(params, kinds[0])
	} else if len(kinds) > 1 {
		conds = append(conds, "kind IN (?)")
		params = append(params, kinds)
	}

	if len(authors) == 1 {
		conds = append(conds, "author = ?")
		params = append(params, authors[0])
	} else if len(authors) > 1 {
		conds = append(conds, "author IN (?)")
		params = append(params, authors)
	}

	if len(sources) == 1 {
		conds = append(conds, "source = ?")
		params = append(params, sources[0])
	} else if len(sources) > 1 {
		conds = append(conds, "source IN (?)")
		params = append(params, sources)
	}

	if !created.IsZero() {
		conds = append(conds, "created >= ?")
		params = append(params, created)
	}
	if !collected.IsZero() {
		conds = append(conds, "collected >= ?")
		params = append(params, collected)
	}
	if !updated.IsZero() {
		conds = append(conds, "updated >= ?")
		params = append(params, updated)
	}

	if len(categories) > 0 {
		conds = append(conds, "ARRAY_HAS_ANY(categories, ?)")
		params = append(params, StringArray(categories))
	}
	if len(regions) > 0 {
		conds = append(conds, "ARRAY_HAS_ANY(regions, ?)")
		params = append(params, StringArray(regions))
	}
	if len(entities) > 0 {
		conds = append(conds, "ARRAY_HAS_ANY(entities, ?)")
		params = append(params, StringArray(entities))
	}

	if distance > 0 {
		conds = append(conds, "distance <= ?")
		params = append(params, distance)
	}
	if len(exprs) > 0 {
		conds = append(conds, exprs...)
	}
	return fmt.Sprintf("WHERE %s", strings.Join(conds, " AND ")), params
}

func limitExpr(limit int64, offset int64) (string, []any) {
	exprs := []string{}
	params := []any{}
	if offset > 0 {
		exprs = append(exprs, "OFFSET ?")
		params = append(params, offset)
	}
	if limit > 0 {
		exprs = append(exprs, "LIMIT ?")
		params = append(params, limit)
	}
	return strings.Join(exprs, " "), params
}

func orderExpr(fields ...string) string {
	if len(fields) > 0 {
		return fmt.Sprintf("ORDER BY %s", strings.Join(fields, ", "))
	}
	return ""
}

////////// QUERY HELPERS //////////

func mustIn(query string, args ...any) (string, []any) {
	query, args, err := sqlx.In(query, args...)
	NoError(err, "SQL error")
	return query, args
}

func shouldIn(query string, args ...any) (string, []any, error) {
	query, args, err := sqlx.In(query, args...)
	LogError(err, "SQL error")
	return query, args, err
}

func mustSelect[T any](db *Beansack, query string, args ...any) []T {
	var data []T
	NoError(db.dbx.Select(&data, query, args...), "SELECT error")
	return data
}

func shouldSelect[T any](db *Beansack, query string, args ...any) ([]T, error) {
	var data []T
	err := db.dbx.Select(&data, query, args...)
	LogError(err, "SELECT error")
	return data, err
}

func getItems[T any](db *Beansack, sql string, ids []string) []T {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := shouldIn(sql, ids)
	if err != nil {
		return nil
	}
	items, err := shouldSelect[T](db, query, args...)
	if err != nil {
		return nil
	}
	return items
}

func (db *Beansack) queryBeans(
	table string,
	urls []string,
	kinds []string,
	authors, sources []string,
	created, collected, updated time.Time,
	categories, regions, entities []string,
	embedding []float32, distance float64,
	where_exprs []string,
	order []string,
	limit int64, offset int64,
	columns []string,
) ([]Bean, error) {
	if len(columns) == 0 {
		columns = []string{PUBLIC_COLUMNS}
	}
	select_expr, select_params := selectExpr(table, columns, embedding)
	where_expr, where_params := whereExpr(urls, kinds, authors, sources, created, collected, updated, categories, regions, entities, embedding, distance, where_exprs)
	if len(embedding) > 0 {
		order = append([]string{ORDER_BY_DISTANCE}, order...)
	}
	order_expr := orderExpr(order...)
	limit_expr, limit_params := limitExpr(limit, offset)

	sql := strings.Join([]string{
		select_expr,
		where_expr,
		order_expr,
		limit_expr,
	}, " ")
	params := make([]any, 0, 10)
	if len(select_params) > 0 {
		params = append(params, select_params...)
	}
	if len(where_params) > 0 {
		params = append(params, where_params...)
	}
	if len(limit_params) > 0 {
		params = append(params, limit_params...)
	}
	// pp.Println(sql)
	sql, params, err := shouldIn(sql, params...)
	if err != nil {
		return nil, err
	}
	beans, err := shouldSelect[Bean](db, sql, params...)
	if err != nil {
		return nil, err
	}
	return beans, nil
}

////////// DIRECT QUERY/GET FUNCTIONS //////////

func (db *Beansack) GetEmbeddings(urls []string) []Bean {
	return getItems[Bean](db, "SELECT * FROM warehouse.bean_embeddings WHERE url IN (?);", urls)
}

func (db *Beansack) GetGists(urls []string) []Bean {
	return getItems[Bean](db, "SELECT * FROM warehouse.bean_gists WHERE url IN (?);", urls)
}

func (db *Beansack) GetRegions(urls []string) []Bean {
	return getItems[Bean](db, `SELECT url, regions FROM warehouse.bean_gists WHERE url IN (?);`, urls)
}

func (db *Beansack) GetEntities(urls []string) []Bean {
	return getItems[Bean](db, `SELECT url, entities FROM warehouse.bean_gists WHERE url IN (?);`, urls)
}

func (db *Beansack) GetCategories(urls []string) []Bean {
	return getItems[Bean](db, `SELECT * FROM warehouse.computed_bean_categories WHERE url IN (?);`, urls)
}

func (db *Beansack) GetSentiments(urls []string) []Bean {
	return getItems[Bean](db, `SELECT * FROM warehouse.computed_bean_sentiments WHERE url IN (?);`, urls)
}

func (db *Beansack) GetRelated(urls []string) []Bean {
	const _SQL_QUERY_CLUSTERS = `
	SELECT url, LIST(DISTINCT related) AS related 
	FROM warehouse.computed_bean_clusters 
	WHERE url IN (?) 
	GROUP BY url;`
	return getItems[Bean](db, _SQL_QUERY_CLUSTERS, urls)
}

func (db *Beansack) GetChatters(urls []string) []Chatter {
	return getItems[Chatter](db, "SELECT * FROM warehouse.chatters WHERE url IN (?);", urls)
}

func (db *Beansack) GetBeanChatters(urls []string) []BeanChatter {
	return getItems[BeanChatter](db, "SELECT * FROM warehouse.bean_chatters_view WHERE url IN (?);", urls)
}

func (db *Beansack) GetSources(urls []string) []Bean {
	return getItems[Bean](db, "SELECT url, source FROM warehouse.bean_cores WHERE url IN (?);", urls)
}

func (db *Beansack) GetPublishers(sources []string) []Publisher {
	return getItems[Publisher](db, "SELECT * FROM warehouse.publishers WHERE source IN (?);", sources)
}

// //////// DISTINCT ITEMS //////////
func (db *Beansack) DistinctRegions() []string {
	const _SQL_GET_ALL_REGIONS = `SELECT DISTINCT unnest(regions) as region FROM warehouse.bean_gists;`
	return mustSelect[string](db, _SQL_GET_ALL_REGIONS)
}

func (db *Beansack) DistinctEntities() []string {
	const _SQL_GET_ALL_ENTITIES = `SELECT DISTINCT unnest(entities) as entity FROM warehouse.bean_gists;`
	return mustSelect[string](db, _SQL_GET_ALL_ENTITIES)
}

func (db *Beansack) DistinctCategories() []string {
	const _SQL_GET_ALL_CATEGORIES = `SELECT DISTINCT unnest(categories) as category FROM warehouse.computed_bean_categories;`
	return mustSelect[string](db, _SQL_GET_ALL_CATEGORIES)
}

func (db *Beansack) DistinctSentiments() []string {
	const _SQL_GET_ALL_SENTIMENTS = `SELECT DISTINCT unnest(sentiments) as sentiment FROM warehouse.computed_bean_sentiments;`
	return mustSelect[string](db, _SQL_GET_ALL_SENTIMENTS)
}

func (db *Beansack) DistinctSources() []string {
	const _SQL_GET_ALL_SOURCES = `SELECT DISTINCT source FROM warehouse.bean_cores;`
	return mustSelect[string](db, _SQL_GET_ALL_SOURCES)
}

// //////// COMPOSITE QUERIES ///////////
func (db *Beansack) QueryLatestBeans(
	urls []string,
	kinds []string,
	authors, sources []string,
	created time.Time,
	categories, regions, entities []string,
	embedding []float32, distance float64,
	where_exprs []string,
	order []string,
	limit int64, offset int64,
	columns []string,
) ([]Bean, error) {
	zero_time := time.Time{}
	return db.queryBeans(
		LATEST_BEANS,
		urls,
		kinds,
		authors, sources,
		created, zero_time, zero_time,
		categories, regions, entities,
		embedding, distance,
		where_exprs,
		append([]string{ORDER_BY_LATEST}, order...),
		limit, offset,
		columns,
	)
}

func (db *Beansack) QueryTrendingBeans(
	urls []string,
	kinds []string,
	authors, sources []string,
	created, updated time.Time,
	categories, regions, entities []string,
	embedding []float32, distance float64,
	where_exprs []string,
	order []string,
	limit int64, offset int64,
	columns []string,
) ([]Bean, error) {
	return db.queryBeans(
		TRENDING_BEANS,
		urls,
		kinds,
		authors, sources,
		created, time.Time{}, updated,
		categories, regions, entities,
		embedding, distance,
		where_exprs,
		append([]string{ORDER_BY_TRENDING}, order...),
		limit, offset,
		columns,
	)
}

func (db *Beansack) Close() error {
	if db == nil || db.dbx == nil {
		return nil
	}
	err := db.dbx.Close()
	if err == nil {
		log.Printf("DB connection closed.")
	}
	return err
}
