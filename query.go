package main

import (
	"strings"

	datautils "github.com/soumitsalman/data-utils"
)

type ExprWithArg struct {
	expr string
	arg  any
}

type SelectExpr struct {
	table   string
	columns []ExprWithArg
	where   []ExprWithArg
	order   []string
	limit   int
	offset  int
	dim     int
}

func NewSelect(ds *Ducksack) *SelectExpr {
	return &SelectExpr{
		table:   "",
		columns: []ExprWithArg{},
		where:   []ExprWithArg{},
		order:   []string{},
		limit:   0,
		offset:  0,
		dim:     ds.dim,
	}
}

func (q *SelectExpr) Table(table string) *SelectExpr {
	q.table = table
	return q
}

func (q *SelectExpr) Columns(cols ...string) *SelectExpr {
	if len(cols) > 0 {
		q.columns = append(
			q.columns,
			datautils.Transform(cols, func(col *string) ExprWithArg {
				return ExprWithArg{expr: *col, arg: nil}
			})...,
		)
	}
	return q
}

func (q *SelectExpr) Where(conditions ...string) *SelectExpr {
	if len(conditions) > 0 {
		q.where = append(
			q.where,
			datautils.Transform(conditions, func(cond *string) ExprWithArg {
				return ExprWithArg{expr: *cond, arg: nil}
			})...,
		)
	}
	return q
}

func (q *SelectExpr) WhereWithArg(expr string, arg any) *SelectExpr {
	q.where = append(q.where, ExprWithArg{expr: expr, arg: arg})
	return q
}

// func (q *SelectExpr) WhereForCustomColumns(
// 	urls []string,
// 	kind string,
// 	created_after time.Time,
// 	categories []string,
// 	regions []string,
// 	entities []string,
// 	sources []string,
// 	embedding []float32,
// 	max_distance float64,
// ) *SelectExpr {
// 	if len(urls) > 0 {
// 		q.where = append(q.where, ExprWithArg{expr: "url IN (?)", arg: urls})
// 	}
// 	if kind != "" {
// 		q.where = append(q.where, ExprWithArg{expr: "kind = ?", arg: kind})
// 	}
// 	if !created_after.IsZero() {
// 		q.where = append(q.where, ExprWithArg{expr: "created >= ?", arg: created_after})
// 	}
// 	if len(categories) > 0 {
// 		q.where = append(q.where, ExprWithArg{expr: "ARRAY_HAS_ANY(categories, ?)", arg: StringArray(categories)})
// 	}
// 	if len(regions) > 0 {
// 		q.where = append(q.where, ExprWithArg{expr: "ARRAY_HAS_ANY(regions, ?)", arg: StringArray(regions)})
// 	}
// 	if len(entities) > 0 {
// 		q.where = append(q.where, ExprWithArg{expr: "ARRAY_HAS_ANY(entities, ?)", arg: StringArray(entities)})
// 	}
// 	if len(sources) > 0 {
// 		q.where = append(q.where, ExprWithArg{expr: "source IN (?)", arg: sources})
// 	}
// 	if embedding != nil {
// 		q.columns = append(q.columns, ExprWithArg{expr: fmt.Sprintf("array_cosine_distance(embedding, ?::FLOAT[%d]) AS distance", q.dim), arg: Float32Array(embedding)})
// 	}
// 	if max_distance > 0 {
// 		q.where = append(q.where, ExprWithArg{expr: "distance <= ?", arg: max_distance})
// 	}
// 	return q
// }

// const _SQL_MISSING_COLUMN = "url NOT IN (SELECT url FROM %s)"

// func (q *SelectExpr) WhereForMissingColumns(columns ...string) *SelectExpr {
// 	for _, column := range columns {
// 		switch column {
// 		case "gist":
// 			q.where = append(q.where, ExprWithArg{expr: fmt.Sprintf(_SQL_MISSING_COLUMN, BEAN_GISTS), arg: nil})
// 		case "embedding":
// 			q.where = append(q.where, ExprWithArg{expr: fmt.Sprintf(_SQL_MISSING_COLUMN, BEAN_EMBEDDINGS), arg: nil})
// 		case "category":
// 			q.where = append(q.where, ExprWithArg{expr: fmt.Sprintf(_SQL_MISSING_COLUMN, BEAN_CATEGORIES), arg: nil})
// 		case "sentiment":
// 			q.where = append(q.where, ExprWithArg{expr: fmt.Sprintf(_SQL_MISSING_COLUMN, BEAN_SENTIMENTS), arg: nil})
// 		case "region":
// 		case "regions":
// 			q.where = append(q.where, ExprWithArg{expr: fmt.Sprintf(_SQL_MISSING_COLUMN, BEAN_REGIONS), arg: nil})
// 		case "entity":
// 		case "entities":
// 			q.where = append(q.where, ExprWithArg{expr: fmt.Sprintf(_SQL_MISSING_COLUMN, BEAN_ENTITIES), arg: nil})
// 		}
// 	}
// 	return q
// }

func (q *SelectExpr) Order(orders ...string) *SelectExpr {
	if len(orders) > 0 {
		q.order = append(q.order, orders...)
	}
	return q
}

func (q *SelectExpr) Limit(limit int) *SelectExpr {
	if limit > 0 {
		q.limit = limit
	}
	return q
}

func (q *SelectExpr) Offset(offset int) *SelectExpr {
	if offset > 0 {
		q.offset = offset
	}
	return q
}

func (q *SelectExpr) ToSQL() (string, []any) {
	var sb strings.Builder
	params := make([]any, 0, len(q.columns)+len(q.where)+len(q.order)+2) // rough estimate

	// SELECT clause
	sb.WriteString("SELECT ")
	if len(q.columns) > 0 {
		// expressions
		cols := datautils.Transform(q.columns, func(col *ExprWithArg) string {
			return col.expr
		})
		sb.WriteString(strings.Join(cols, ", "))
		// params
		for _, col := range q.columns {
			if col.arg != nil {
				params = append(params, col.arg)
			}
		}

	} else {
		sb.WriteString("*")
	}

	// FROM clause
	if q.table != "" {
		sb.WriteString(" FROM ")
		sb.WriteString(q.table)
	}

	// WHERE clause
	if len(q.where) > 0 {
		sb.WriteString(" WHERE ")
		// expressions
		whereExprs := datautils.Transform(q.where, func(cond *ExprWithArg) string {
			return cond.expr
		})
		sb.WriteString(strings.Join(whereExprs, " AND "))
		// params
		for _, cond := range q.where {
			if cond.arg != nil {
				params = append(params, cond.arg)
			}
		}

	}

	// ORDER BY clause
	if len(q.order) > 0 {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(strings.Join(q.order, ", "))
	}

	// LIMIT clause
	if q.limit > 0 {
		sb.WriteString(" LIMIT ?")
		params = append(params, q.limit)
	}

	// OFFSET clause
	if q.offset > 0 {
		sb.WriteString(" OFFSET ?")
		params = append(params, q.offset)
	}

	return sb.String(), params
}
