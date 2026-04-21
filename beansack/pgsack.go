package beansack

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
	pgxvec "github.com/pgvector/pgvector-go/pgx"
	datautils "github.com/soumitsalman/data-utils"
)

const (
	_TIMEOUT        = 10
	_POOL_SIZE      = 32
	_CONN_LIFETIME  = 5
	_CONN_IDLE_TIME = 5
)

const (
	_TRENDING_BEANS_VIEW   = "trending_beans_view"
	_AGGREGATED_BEANS_VIEW = "aggregated_beans_view"
)

type PGSack struct {
	db *pgxpool.Pool
}

func NewPGSack(ctx context.Context, connString string) *PGSack {
	config, err := pgxpool.ParseConfig(connString)
	NoError(err)

	config.MinConns = 1
	config.MaxConns = _POOL_SIZE
	config.MaxConnLifetime = time.Minute * _CONN_LIFETIME
	config.MaxConnIdleTime = time.Minute * _CONN_IDLE_TIME
	config.HealthCheckPeriod = time.Minute * _CONN_LIFETIME
	config.ConnConfig.ConnectTimeout = time.Minute * _TIMEOUT
	if config.ConnConfig.RuntimeParams == nil {
		config.ConnConfig.RuntimeParams = map[string]string{}
	}
	config.ConnConfig.RuntimeParams["statement_timeout"] = fmt.Sprintf("%dmin", _TIMEOUT)
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return pgxvec.RegisterTypes(ctx, conn)
	}

	db, err := pgxpool.NewWithConfig(ctx, config)
	NoError(err)
	NoError(db.Ping(ctx)) // Quick health check on startup

	return &PGSack{db: db}
}

func (p *PGSack) QueryBeans(ctx context.Context, conditions Condition, page Pagination, columns []string) ([]BeanAggregate, error) {
	items, err := fetchBeans(ctx, p, _AGGREGATED_BEANS_VIEW, conditions, nil, page, columns)
	if err != nil {
		return nil, err
	}
	return datautils.Transform(items, func(item *dataRow) BeanAggregate { return item.toBeanAggregate() }), nil
}

func (p *PGSack) QueryLatestBeans(ctx context.Context, conditions Condition, page Pagination, columns []string) ([]Bean, error) {
	items, err := fetchBeans(ctx, p, BEANS, conditions, []string{ORDER_BY_LATEST}, page, columns)
	if err != nil {
		return nil, err
	}
	return datautils.Transform(items, func(item *dataRow) Bean { return item.toBean() }), nil
}

func (p *PGSack) QueryTrendingBeans(ctx context.Context, conditions Condition, page Pagination, columns []string) ([]BeanTrend, error) {
	items, err := fetchBeans(ctx, p, _TRENDING_BEANS_VIEW, conditions, []string{ORDER_BY_TRENDING}, page, columns)
	if err != nil {
		return nil, err
	}
	return datautils.Transform(items, func(item *dataRow) BeanTrend { return item.toBeanTrend() }), nil
}

// TODO: how to pass in text/tag/keyword based search
func (p *PGSack) QueryPublishers(ctx context.Context, conditions Condition, page Pagination, columns []string) ([]Publisher, error) {
	query, args := p.buildSQL(PUBLISHERS, conditions, nil, page, columns)
	items, err := fetchAll[dataRow](ctx, p.db, query, args)
	if err != nil {
		return nil, err
	}
	return datautils.Transform(items, func(item *dataRow) Publisher { return item.toPublisher() }), nil
}

func (p *PGSack) QueryChatters(ctx context.Context, conditions Condition, page Pagination, columns []string) ([]Chatter, error) {
	query, args := p.buildSQL(CHATTERS, conditions, nil, page, columns)
	items, err := fetchAll[dataRow](ctx, p.db, query, args)
	if err != nil {
		return nil, err
	}
	return datautils.Transform(items, func(item *dataRow) Chatter { return item.toChatter() }), nil
}

func (p *PGSack) DistinctCategories(ctx context.Context, page Pagination) ([]string, error) {
	// SELECT DISTINCT unnest(categories) AS category FROM beans WHERE categories IS NOT NULL ORDER BY category
	query, args := p.buildSQL(BEANS, Condition{Extra: []string{"categories IS NOT NULL"}}, []string{"category"}, page, []string{"DISTINCT unnest(categories) AS category"})
	return fetchAllScalar[string](ctx, p.db, query, args)
}

func (p *PGSack) DistinctSentiments(ctx context.Context, page Pagination) ([]string, error) {
	query, args := p.buildSQL(BEANS, Condition{Extra: []string{"sentiments IS NOT NULL"}}, []string{"sentiment"}, page, []string{"DISTINCT unnest(sentiments) AS sentiment"})
	return fetchAllScalar[string](ctx, p.db, query, args)
}

func (p *PGSack) DistinctEntities(ctx context.Context, page Pagination) ([]string, error) {
	query, args := p.buildSQL(BEANS, Condition{Extra: []string{"entities IS NOT NULL"}}, []string{"entity"}, page, []string{"DISTINCT unnest(entities) AS entity"})
	return fetchAllScalar[string](ctx, p.db, query, args)
}

func (p *PGSack) DistinctRegions(ctx context.Context, page Pagination) ([]string, error) {
	query, args := p.buildSQL(BEANS, Condition{Extra: []string{"regions IS NOT NULL"}}, []string{"region"}, page, []string{"DISTINCT unnest(regions) AS region"})
	return fetchAllScalar[string](ctx, p.db, query, args)
}

func (p *PGSack) DistinctSources(ctx context.Context, page Pagination) ([]string, error) {
	query, args := p.buildSQL(PUBLISHERS, Condition{}, []string{"source"}, page, []string{"source"})
	return fetchAllScalar[string](ctx, p.db, query, args)
}

func (p *PGSack) CountRows(ctx context.Context, table string, conditions Condition) (int64, error) {
	query, args := p.buildSQL(table, conditions, nil, Pagination{}, []string{"count(*)"})
	return fetchOneScalar[int64](ctx, p.db, query, args)
}

func (p *PGSack) Close() {
	if p != nil && p.db != nil {
		p.db.Close()
	}
}

func fetchBeans(ctx context.Context, p *PGSack, table string, conditions Condition, orders []string, page Pagination, columns []string) ([]dataRow, error) {
	query, args := p.buildSQL(table, conditions, orders, page, columns)
	beans, err := fetchAll[dataRow](ctx, p.db, query, args)
	// logQueryResult(beans, err)
	return beans, err
}

// SQL query string builder utilities
// TODO: add function for building query
func (p *PGSack) buildSQL(table string, conditions Condition, orders []string, page Pagination, columns []string) (string, pgx.NamedArgs) {
	// where clause first - because we may need it before select
	where_expr, where_params := p.buildSQLWhere(conditions)

	// select fields
	fields := "*"
	if len(columns) > 0 {
		fields = strings.Join(columns, ", ")
	}

	// either simple select or vector search with distance calculation
	base_expr := fmt.Sprintf("SELECT %s FROM %s %s", fields, table, where_expr)
	base_params := pgx.NamedArgs{}
	if conditions.Embedding != nil {
		base_expr = fmt.Sprintf(`
			WITH vector_distances AS (
                SELECT *, (embedding <=> @embedding::vector) AS distance
                FROM %s
				%s
            )
            SELECT %s
            FROM vector_distances
            WHERE distance <= @distance`,
			table, where_expr, fields,
		)
		base_params["embedding"] = pgvector.NewVector(conditions.Embedding)
		base_params["distance"] = conditions.Distance
		if orders == nil {
			orders = []string{ORDER_BY_DISTANCE}
		} else {
			orders = append(orders, ORDER_BY_DISTANCE)
		}
	}
	builder := strings.Builder{}
	builder.WriteString(base_expr)

	// orders
	if len(orders) > 0 {
		builder.WriteString(" ")
		builder.WriteString(p.buildPGOrderBy(orders...))
	}
	// pagination
	page_expr, page_params := p.buildPGLimitOffset(page)
	if page_expr != "" {
		builder.WriteString(" ")
		builder.WriteString(page_expr)
	}
	query, args := builder.String(), mergeParams(base_params, where_params, page_params)
	// LogQuery(query, args)
	return query, args
}

func mergeParams(maps ...pgx.NamedArgs) pgx.NamedArgs {
	merged := pgx.NamedArgs{}
	for _, m := range maps {
		if m != nil {
			for k, v := range m {
				merged[k] = v
			}
		}
	}
	return merged
}

func (p *PGSack) buildSQLWhere(conditions Condition) (string, pgx.NamedArgs) {
	parts := make([]string, 0, 10) // preallocate for expected conditions
	args := pgx.NamedArgs{}

	if len(conditions.URLs) > 0 {
		parts = append(parts, "url = ANY(@urls)")
		args["urls"] = conditions.URLs // pgx handles []string as array automatically
	}

	if conditions.Kind != "" {
		parts = append(parts, "kind = @kind")
		args["kind"] = conditions.Kind
	}

	if !conditions.Created.IsZero() {
		parts = append(parts, "created >= @created_from")
		args["created_from"] = conditions.Created
	}

	if !conditions.Collected.IsZero() {
		parts = append(parts, "collected >= @collected_from")
		args["collected_from"] = conditions.Collected
	}

	if !conditions.Updated.IsZero() {
		parts = append(parts, "updated >= @updated_from")
		args["updated_from"] = conditions.Updated
	}

	if len(conditions.Categories) > 0 {
		parts = append(parts, "categories && @categories")
		args["categories"] = conditions.Categories
	}

	if len(conditions.Regions) > 0 {
		parts = append(parts, "regions && @regions")
		args["regions"] = conditions.Regions
	}

	if len(conditions.Entities) > 0 {
		parts = append(parts, "entities && @entities")
		args["entities"] = conditions.Entities
	}

	if len(conditions.Tags) > 0 {
		parts = append(parts, "tags @@ plainto_tsquery('simple', @tags_query)")
		args["tags_query"] = strings.Join(conditions.Tags, " & ") // "tag1 & tag2 & tag3"
	}

	if len(conditions.Sources) > 0 {
		parts = append(parts, "source = ANY(@sources)")
		args["sources"] = conditions.Sources
	}

	if len(conditions.Extra) > 0 {
		parts = append(parts, conditions.Extra...)
	}

	if len(parts) == 0 {
		return "", nil
	}
	return fmt.Sprintf("WHERE %s", strings.Join(parts, " AND ")), args
}

func (p *PGSack) buildPGOrderBy(order ...string) string {
	if len(order) == 0 {
		return ""
	}
	return "ORDER BY " + strings.Join(order, ", ")
}

func (p *PGSack) buildPGLimitOffset(page Pagination) (string, pgx.NamedArgs) {
	parts := make([]string, 0, 2)
	args := pgx.NamedArgs{}

	if page.Limit > 0 {
		parts = append(parts, "LIMIT @limit")
		args["limit"] = page.Limit
	}
	if page.Offset > 0 {
		parts = append(parts, "OFFSET @offset")
		args["offset"] = page.Offset
	}

	if len(parts) == 0 {
		return "", nil
	}
	return strings.Join(parts, " "), args
}

// PGX fetch helpers
func fetchOne[T any](ctx context.Context, db *pgxpool.Pool, query string, args pgx.NamedArgs) (T, error) {
	rows, err := db.Query(ctx, query, args)
	if err != nil {
		var zero T
		return zero, err
	}
	defer rows.Close()
	return pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[T])
}

func fetchOneScalar[T any](ctx context.Context, db *pgxpool.Pool, query string, args pgx.NamedArgs) (T, error) {
	rows, err := db.Query(ctx, query, args)
	if err != nil {
		var zero T
		return zero, err
	}
	defer rows.Close()
	return pgx.CollectOneRow(rows, pgx.RowTo[T])
}

func fetchAll[T any](ctx context.Context, db *pgxpool.Pool, query string, args pgx.NamedArgs) ([]T, error) {
	rows, err := db.Query(ctx, query, args)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByNameLax[T])
}

func fetchAllScalar[T any](ctx context.Context, db *pgxpool.Pool, query string, args pgx.NamedArgs) ([]T, error) {
	rows, err := db.Query(ctx, query, args)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowTo[T])
}

// PGX marshalling and unmarshalling for custom types

type dataRow struct {
	URL         sql.NullString  `db:"url"`
	Kind        sql.NullString  `db:"kind"`
	Title       sql.NullString  `db:"title"`
	Summary     sql.NullString  `db:"summary"`
	Content     sql.NullString  `db:"content"`
	Author      sql.NullString  `db:"author"`
	Source      sql.NullString  `db:"source"`
	ImageUrl    sql.NullString  `db:"image_url"`
	Created     sql.NullTime    `db:"created"`
	Embedding   pgvector.Vector `db:"embedding"`
	Gist        sql.NullString  `db:"gist"`
	Categories  []string        `db:"categories"`
	Sentiments  []string        `db:"sentiments"`
	Regions     []string        `db:"regions"`
	Entities    []string        `db:"entities"`
	Related     sql.NullInt64   `db:"related"`
	ClusterSize sql.NullInt64   `db:"cluster_size"`
	Updated     sql.NullTime    `db:"updated"`
	Likes       sql.NullInt64   `db:"likes"`
	Comments    sql.NullInt64   `db:"comments"`
	Subscribers sql.NullInt64   `db:"subscribers"`
	Shares      sql.NullInt64   `db:"shares"`
	Distance    float64         `db:"distance"`
	TrendScore  float64         `db:"trend_score"`
	ChatterURL  sql.NullString  `db:"chatter_url"`
	Forum       sql.NullString  `db:"forum"`
	Collected   sql.NullTime    `db:"collected"`
	BaseURL     sql.NullString  `db:"base_url"`
	SiteName    sql.NullString  `db:"site_name"`
	Description sql.NullString  `db:"description"`
	Favicon     sql.NullString  `db:"favicon"`
	RSSFeed     sql.NullString  `db:"rss_feed"`
}

// Conversion methods from dataRow to public types
func (r *dataRow) toBean() Bean {
	return Bean{
		URL:        r.URL.String,
		Kind:       r.Kind.String,
		Title:      r.Title.String,
		Summary:    r.Summary.String,
		Content:    r.Content.String,
		Author:     r.Author.String,
		Source:     r.Source.String,
		ImageUrl:   r.ImageUrl.String,
		Created:    r.Created.Time,
		Embedding:  r.Embedding.Slice(),
		Gist:       r.Gist.String,
		Categories: r.Categories,
		Sentiments: r.Sentiments,
		Regions:    r.Regions,
		Entities:   r.Entities,
		MergedTags: ConcatArray[string](r.Categories, r.Regions, r.Entities),
	}
}

func (r *dataRow) toBeanTrend() BeanTrend {
	return BeanTrend{
		Bean:        r.toBean(),
		Likes:       r.Likes.Int64,
		Comments:    r.Comments.Int64,
		Subscribers: r.Subscribers.Int64,
		Shares:      r.Shares.Int64,
		Related:     r.Related.Int64,
		Updated:     r.Updated.Time,
		TrendScore:  r.TrendScore,
	}
}

func (r *dataRow) toBeanAggregate() BeanAggregate {
	return BeanAggregate{
		BeanTrend:   r.toBeanTrend(),
		BaseURL:     r.BaseURL.String,
		SiteName:    r.SiteName.String,
		Description: r.Description.String,
		Favicon:     r.Favicon.String,
	}
}

func (r *dataRow) toPublisher() Publisher {
	return Publisher{
		Source:      r.Source.String,
		BaseURL:     r.BaseURL.String,
		SiteName:    r.SiteName.String,
		Description: r.Description.String,
		Favicon:     r.Favicon.String,
		RSSFeed:     r.RSSFeed.String,
		Collected:   r.Collected.Time,
	}
}

func (r *dataRow) toChatter() Chatter {
	return Chatter{
		ChatterURL:  r.ChatterURL.String,
		URL:         r.URL.String,
		Source:      r.Source.String,
		Forum:       r.Forum.String,
		Collected:   r.Collected.Time,
		Likes:       r.Likes.Int64,
		Comments:    r.Comments.Int64,
		Subscribers: r.Subscribers.Int64,
	}
}

func (r *dataRow) toChatterAggregate() ChatterAggregate {
	return ChatterAggregate{
		URL:         r.URL.String,
		Collected:   r.Collected.Time,
		Likes:       r.Likes.Int64,
		Comments:    r.Comments.Int64,
		Subscribers: r.Subscribers.Int64,
		Shares:      r.Shares.Int64,
	}
}
