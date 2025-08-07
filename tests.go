package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/k0kubun/pp"
	datautils "github.com/soumitsalman/data-utils"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	DEFAULT_DB_NAME  = "master"
	DEFAULT_CONN_STR = "mongodb://localhost:27017"
)

var testCtx = context.Background()

func setupTestDB(t *testing.T) *Ducksack {
	// Load configuration from environment variables
	err := godotenv.Load(".env")
	if err != nil {
		t.Skip("Skipping test: .env file not found")
	}

	dbpath, ok := os.LookupEnv("DB_PATH")
	if !ok {
		dbpath = DEFAULT_DB_PATH
	}
	dim := DEFAULT_VECTOR_DIM
	cluster_eps := DEFAULT_CLUSTER_EPS

	// initialize database if needed
	init, err := os.ReadFile("./factory/init.sql")
	if err != nil {
		t.Fatal("Failed to read init.sql:", err)
	}
	initsql := string(init)

	ds := NewBeansack(dbpath, initsql, dim, cluster_eps)
	t.Cleanup(func() {
		ds.Close()
	})

	return ds
}

func TestQueryDistinctValues(t *testing.T) {
	ds := setupTestDB(t)
	pp.Println("REGIONS", ds.DistinctRegions()[:5])
	pp.Println("ENTITIES", ds.DistinctEntities()[:5])
	pp.Println("SOURCES", ds.DistinctSources()[:5])
	pp.Println("CATEGORIES", ds.DistinctCategories()[:5])
}

func TestQueryChatterStats(t *testing.T) {
	ds := setupTestDB(t)
	urls := []string{
		"https://www.slashgear.com/1896648/lifesaber-emergency-tool-usb-powered-features/",
		"https://www.wusa9.com/article/news/nation-world/trump-big-bill-may-have-political-cost/507-0c07fc4b-248b-4a3b-96c0-81d75a511228",
		"https://issuepay.app",
		"https://minutemirror.com.pk/lahore-high-court-halts-transfer-of-brazilian-monkeys-to-lahore-zoo-406544/",
		"https://jameshard.ing/pilot",
		"https://jobsbyreferral.com/",
		"https://llmapitest.com/",
		"https://htmlrev.com/",
	}

	pp.Println("CHATTERS", ds.GetChatters(urls)[:5])
	pp.Println("AGGREGATES", ds.GetBeanChatters(urls))
}

func TestQueryBeanExtensions(t *testing.T) {
	ds := setupTestDB(t)
	urls := []string{
		"https://www.slashgear.com/1896648/lifesaber-emergency-tool-usb-powered-features/",
		"https://www.wusa9.com/article/news/nation-world/trump-big-bill-may-have-political-cost/507-0c07fc4b-248b-4a3b-96c0-81d75a511228",
		"https://issuepay.app",
		"https://minutemirror.com.pk/lahore-high-court-halts-transfer-of-brazilian-monkeys-to-lahore-zoo-406544/",
		"https://jameshard.ing/pilot",
		"https://jobsbyreferral.com/",
		"https://llmapitest.com/",
		"https://htmlrev.com/",
	}

	pp.Println("CATEGORIES", ds.GetCategories(urls))
	pp.Println("SENTIMENTS", ds.GetSentiments(urls))
	pp.Println("RELATED", ds.GetRelated(urls))
	pp.Println("REGIONS", ds.GetRegions(urls))
	pp.Println("ENTITIES", ds.GetEntities(urls))
	pp.Println("GISTS", ds.GetGists(urls))
	pp.Println("BEAN AGGREGATES", ds.GetBeans(urls))
	pp.Println("BEAN CLUSTERS", ds.GetRelated(urls))
}

func TestVectorSearch(t *testing.T) {
	ds := setupTestDB(t)
	query_emb := Float32Array{
		// ... embedding values from original test
	}

	sources := []string{"techstartups", "techradar"}
	beans, err := ds.VectorSearchBeansWithSelectFields(query_emb, 0.25, NEWS, time.Time{}, nil, nil, nil, sources, nil, 0, 5, nil)
	noerror(err, "VECTOR SEARCH ERROR")
	datautils.PrintTable(
		beans,
		[]string{"kind", "title", "categories", "entities", "created", "source"},
		func(b *Bean) []string {
			return []string{b.Kind, b.Title, strings.Join(b.Categories, ", "), strings.Join(b.Entities, ", "), b.Created.Format(time.RFC3339), b.Source}
		},
	)
}

func TestQueryBeans(t *testing.T) {
	ds := setupTestDB(t)
	categories := []string{"Artificial Intelligence", "Cloud Computing"}
	entities := []string{"ChatGPT", "Elon Musk"}

	beans, err := ds.QueryBeansWithSelectFields(NEWS, time.Now().AddDate(0, 0, -3), categories, nil, entities, nil, nil, 0, 5, nil)
	noerror(err, "QUERY BEANS ERROR")
	for i := int64(0); i < 3; i++ {
		datautils.PrintTable(
			beans,
			[]string{"kind", "title", "categories", "entities", "created", "source"},
			func(b *Bean) []string {
				return []string{b.Kind, b.Title, strings.Join(b.Categories, ", "), strings.Join(b.Entities, ", "), b.Created.Format(time.RFC3339), b.Source}
			},
		)
	}
}

func TestQueryRelated(t *testing.T) {
	ds := setupTestDB(t)
	urls := []string{
		"https://decrypt.co/328057/cloudflare-hits-kill-switch-ai-crawlers-entire-industry-cheers",
		"https://uk.pcmag.com/ai/158866/cloudflare-to-block-ai-crawlers-from-scraping-websites-unless-they-pay",
		"https://www.pcmag.com/news/cloudflare-to-block-ai-crawlers-from-scraping-websites-unless-they-pay",
		"https://www.engadget.com/ai/cloudflare-experiment-will-block-ai-bot-scrapers-unless-they-pay-a-fee-1215233…",
		"https://dailygalaxy.com/2025/07/red-jellyfish-bursts-storm-outer-space/",
	}
	pp.Println(ds.GetRelated(urls))
}

// Test data generation functions
func getTestMongoCollection(collection string) *mongo.Collection {
	connstr := os.Getenv("MONGODB_CONN_STR")
	if connstr == "" {
		connstr = DEFAULT_CONN_STR
	}
	fmt.Println(connstr)
	client, err := mongo.Connect(testCtx, options.Client().ApplyURI(connstr))
	if err != nil {
		panic(fmt.Sprintf("MONGO CONNECT ERROR: %v", err))
	}
	dbname := os.Getenv("DB_NAME")
	fmt.Println(dbname)
	if dbname == "" {
		dbname = DEFAULT_DB_NAME
	}
	db := client.Database(dbname)
	return db.Collection(collection)
}

func testMongoFind[T any](collection string, filter interface{}, skip int64, limit int64, projection interface{}) []T {
	if filter == nil {
		filter = map[string]interface{}{}
	}
	if projection == nil {
		projection = map[string]interface{}{}
	}
	coll := getTestMongoCollection(collection)
	defer coll.Database().Client().Disconnect(testCtx)

	find_options := options.Find().SetSkip(skip).SetLimit(limit).SetProjection(projection)
	cursor, err := coll.Find(testCtx, filter, find_options)
	if err != nil {
		panic(fmt.Sprintf("MONGO FIND ERROR: %v", err))
	}
	defer cursor.Close(testCtx)
	items := []T{}
	for cursor.Next(testCtx) {
		var item T
		err := cursor.Decode(&item)
		if err != nil {
			panic(fmt.Sprintf("MONGO DECODE ERROR: %v", err))
		}
		items = append(items, item)
	}
	return items
}

func TestHydrateDB(t *testing.T) {
	ds := setupTestDB(t)
	var importnum int64 = 10000

	ds.StoreChatters(getTestChatters(0, 400000))

	for i := int64(0); i < 25; i++ {
		beans := getTestBeans(i*importnum, importnum)
		ds.StoreBeans(beans)
		ds.StoreTags(beans)
		embs := datautils.Filter(beans, func(b *Bean) bool {
			return len(b.Embedding) > 0
		})
		ds.StoreEmbeddings(embs)

	}
}

func getTestBeans(skip int64, limit int64) []Bean {
	beans := testMongoFind[Bean](
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
	return beans
}

func getTestEmbeddings(skip int64, limit int64) []EmbeddingData {
	return testMongoFind[EmbeddingData](
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

func getTestChatters(skip int64, limit int64) []Chatter {
	return testMongoFind[Chatter](
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
}

func getTestSources(skip int64, limit int64) []Source {
	return testMongoFind[Source]("sources", nil, skip, limit, nil)
}

type TestDigest struct {
	URL        string   `bson:"url"`
	Categories []string `bson:"categories"`
	Sentiments []string `bson:"sentiments"`
	Gist       string   `bson:"gist"`
	Regions    []string `bson:"regions"`
	Entities   []string `bson:"entities"`
}

func getTestDigests(skip int64, limit int64) []TestDigest {
	return testMongoFind[TestDigest](
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
}
