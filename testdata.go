package main

import (
	"context"
	"fmt"
	"os"

	datautils "github.com/soumitsalman/data-utils"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	DEFAULT_DB_NAME  = "master"
	DEFAULT_CONN_STR = "mongodb://localhost:27017"
)

var ctx = context.Background()

func hydrateTestDB(ds *Ducksack) {
	var importnum int64 = 10000

	ds.StoreChatters(getTestChatters(0, 400000))

	for i := int64(0); i < 25; i++ {
		beans := getTestBeans(i*importnum, importnum)
		ds.StoreBeans(beans)
		embs := datautils.Filter(beans, func(b *Bean) bool {
			return len(b.Embedding) > 0
		})
		ds.StoreEmbeddings(embs)

		tags := prepareTags(beans)

		if categories, ok := tags["categories"]; ok && len(categories) > 0 {
			ds.StoreTags(categories, BEAN_CATEGORIES)
		}
		if sentiments, ok := tags["sentiments"]; ok && len(sentiments) > 0 {
			ds.StoreTags(sentiments, BEAN_SENTIMENTS)
		}
		if regions, ok := tags["regions"]; ok && len(regions) > 0 {
			ds.StoreTags(regions, BEAN_REGIONS)
		}
		if entities, ok := tags["entities"]; ok && len(entities) > 0 {
			ds.StoreTags(entities, BEAN_ENTITIES)
		}
		if gist, ok := tags["gist"]; ok && len(gist) > 0 {
			ds.StoreTags(gist, BEAN_GISTS)
		}
	}

	// ds.RectifyExtendedFields(beans, 3, 0.43)

	// ds.StoreSources(getTestSources(importnum))
	// digests := getTestDigests(importnum)
}

func getMongoCollection(collection string) *mongo.Collection {
	connstr := os.Getenv("MONGODB_CONN_STR")
	if connstr == "" {
		connstr = DEFAULT_CONN_STR
	}
	fmt.Println(connstr)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(connstr))
	noerror(err, "MONGO CONNECT ERROR")
	dbname := os.Getenv("DB_NAME")
	fmt.Println(dbname)
	if dbname == "" {
		dbname = DEFAULT_DB_NAME
	}
	db := client.Database(dbname)
	return db.Collection(collection)
}

func mongoFind[T any](collection string, filter interface{}, skip int64, limit int64, projection interface{}) []T {
	if filter == nil {
		filter = map[string]interface{}{}
	}
	if projection == nil {
		projection = map[string]interface{}{}
	}
	coll := getMongoCollection(collection)
	defer coll.Database().Client().Disconnect(ctx)

	find_options := options.Find().SetSkip(skip).SetLimit(limit).SetProjection(projection)
	cursor, err := coll.Find(ctx, filter, find_options)
	noerror(err, "MONGO FIND ERROR")
	defer cursor.Close(ctx)
	items := []T{}
	for cursor.Next(ctx) {
		var item T
		err := cursor.Decode(&item)
		noerror(err, "MONGO DECODE ERROR")
		items = append(items, item)
	}
	return items
}

func getTestBeans(skip int64, limit int64) []Bean {
	beans := mongoFind[Bean](
		"beans",
		map[string]interface{}{
			"content": map[string]interface{}{
				"$exists": true,
			},
			"summary": map[string]interface{}{
				"$exists": true,
			},
		},
		skip,
		limit,
		map[string]interface{}{
			"url":        1,
			"title":      1,
			"content":    1,
			"summary":    1,
			"created":    1,
			"updated":    1,
			"collected":  1,
			"kind":       1,
			"source":     1,
			"author":     1,
			"embedding":  1,
			"categories": 1,
			"sentiments": 1,
			"regions":    1,
			"entities":   1,
			"gist":       1,
		},
	)
	for i := range beans {
		if len(beans[i].MongoEmbedding) > 0 {
			beans[i].Embedding.Scan(beans[i].MongoEmbedding)
		}
	}
	return beans
}

func getTestEmbeddings(skip int64, limit int64) []EmbeddingData {
	return mongoFind[EmbeddingData](
		"beans",
		map[string]interface{}{
			"embedding": map[string]interface{}{
				"$exists": true,
			},
		},
		skip,
		limit,
		map[string]interface{}{
			"url":       1,
			"embedding": 1,
		},
	)
}

type Digest struct {
	URL        string   `bson:"url"`
	Categories []string `bson:"categories"`
	Sentiments []string `bson:"sentiments"`
	Gist       string   `bson:"gist"`
	Regions    []string `bson:"regions"`
	Entities   []string `bson:"entities"`
}

func getTestDigests(skip int64, limit int64) []Digest {
	digests := mongoFind[Digest](
		"beans",
		map[string]interface{}{
			"gist": map[string]interface{}{
				"$exists": true,
			},
		},
		skip,
		limit,
		map[string]interface{}{
			"url":        1,
			"gist":       1,
			"regions":    1,
			"entities":   1,
			"categories": 1,
			"sentiments": 1,
		},
	)
	return digests
}

func getTestChatters(skip int64, limit int64) []Chatter {
	chatters := mongoFind[Chatter](
		"chatters",
		map[string]interface{}{
			"likes": map[string]interface{}{
				"$exists": true,
			},
			"comments": map[string]interface{}{
				"$exists": true,
			},
		},
		skip,
		limit,
		nil,
	)
	// fmt.Println(chatters[3])
	return chatters
}

func getTestSources(skip int64, limit int64) []Source {
	sources := mongoFind[Source]("sources", nil, skip, limit, nil)
	return sources
}
