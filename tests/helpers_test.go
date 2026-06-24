package gobeansack_test

import (
	"context"
	"os"
	"time"

	"github.com/joho/godotenv"
	bs "github.com/soumitsalman/beansapi/beansack"
	"github.com/soumitsalman/beansapi/nlp"
)

const testVectorQuery = "market trend changes due to public policy changes"

var (
	testCategories = []string{"public_policy_and_administration", "art_and_design", "cybersecurity"}
	testRegions    = []string{"north_america", "europe", "united_states"}
	testEntities   = []string{"openai", "microsoft", "google"}
	testTags       = []string{"artificial_intelligence", "cybersecurity"}
	testSources    = []string{"techcrunch", "slashgear"}
	testPropagationURLs = []string{
		"https://www.foxla.com/news/more-us-airlines-raise-baggage-fees-see-list",
		"https://www.slashgear.com/2143752/us-airlines-increased-fees-fuel-prices/",
		"http://andersource.dev/2026/03/29/tradeoff-sliders.html",
		"https://massivelyop.com/2026/04/08/perfect-ten-have-official-mmo-websites-improved-in-the-last-decade/",
		"https://phys.org/news/2026-04-uncharted-island-nautical.html",
	}
)

func setupTestDB() bs.Beansack {
	bs.NoError(godotenv.Load("../.env"))
	connStr := os.Getenv("PG_CONNECTION_STRING")
	return bs.NewPGSack(context.Background(), connStr)
}

func setupTestEmbedder() *nlp.RemoteEmbedder {
	bs.NoError(godotenv.Load("../.env"))
	return nlp.NewRemoteEmbedder(
		os.Getenv("EMBEDDER_BASE_URL"),
		os.Getenv("EMBEDDER_API_KEY"),
		os.Getenv("EMBEDDER_MODEL"),
	)
}

func testSearchFrom() time.Time {
	return time.Now().UTC().AddDate(0, 0, -7)
}
