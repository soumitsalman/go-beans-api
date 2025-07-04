package main

import (
	"fmt"
	"os"

	"github.com/k0kubun/pp"
	datautils "github.com/soumitsalman/data-utils"
)

func flattenTags(url string, data []string) []TagData {
	var tags []TagData = make([]TagData, 0, len(data))
	for _, tag := range data {
		tags = append(tags, TagData{URL: url, Tag: tag})
	}
	return tags
}

func prepareTags(digests []Digest) map[string][]TagData {
	// rough initialization
	results := map[string][]TagData{
		"categories": make([]TagData, 0, 3*len(digests)),
		"sentiments": make([]TagData, 0, 3*len(digests)),
		"regions":    make([]TagData, 0, 3*len(digests)),
		"entities":   make([]TagData, 0, 3*len(digests)),
		"gist":       make([]TagData, 0, len(digests)),
	}
	for _, digest := range digests {
		results["categories"] = append(results["categories"], flattenTags(digest.URL, digest.Categories)...)
		results["sentiments"] = append(results["sentiments"], flattenTags(digest.URL, digest.Sentiments)...)
		results["regions"] = append(results["regions"], flattenTags(digest.URL, digest.Regions)...)
		results["entities"] = append(results["entities"], flattenTags(digest.URL, digest.Entities)...)
		results["gist"] = append(results["gist"], TagData{URL: digest.URL, Tag: digest.Gist})
	}
	return results
}

func main() {
	// initialize database if needed
	init, err := os.ReadFile("./factory/init.sql")
	noerror(err)
	dim := 384
	dbpath := ".cache/test.db"
	initsql := fmt.Sprintf(string(init), dim, dim, "./factory/categories.parquet", dim, "./factory/sentiments.parquet")

	ds := NewDucksack(dbpath, initsql, dim)
	defer ds.Close()

	var importnum int64 = 400
	ds.StoreBeans(getTestBeans(importnum))
	ds.StoreEmbeddings(getTestEmbeddings(importnum))
	ds.StoreChatters(getTestChatters(400000))
	ds.StoreSources(getTestSources(importnum))
	digests := getTestDigests(importnum)
	tags := prepareTags(digests)

	if regions, ok := tags["regions"]; ok && len(regions) > 0 {
		ds.StoreTags(regions, BEAN_REGIONS)
	}
	if entities, ok := tags["entities"]; ok && len(entities) > 0 {
		ds.StoreTags(entities, BEAN_ENTITIES)
	}
	if gist, ok := tags["gist"]; ok && len(gist) > 0 {
		ds.StoreTags(gist, BEAN_GISTS)
	}
	if cats, ok := tags["categories"]; ok && len(cats) > 0 {
		ds.StoreTags(cats, BEAN_CATEGORIES)
	}
	if sentiments, ok := tags["sentiments"]; ok && len(sentiments) > 0 {
		ds.StoreTags(sentiments, BEAN_SENTIMENTS)
	}

	// titles := []string{
	// 	"Synthflow AI is bringing 'conversational' voice agents to call centers. Read the pitch deck that it used to raise $20 million.",
	// 	"Lahore high court halts transfer of Brazilian monkeys to Lahore zoo",
	// 	"Introducing the Klaviyo Partner Portal: our newest investment in helping our ecosystem grow with us",
	// 	"25 journalists jailed in Turkey in first quarter of 2025: report",
	// 	"KP police gets advanced anti drone tech to target terrorist drones",
	// }
	// urls := []string{
	// 	"https://greekreporter.com/2025/06/25/top-20-greek-islands-travel-greece/",
	// 	"https://www.businessinsider.com/synthflow-ai-pitch-deck-funding-voice-2025-6",
	// 	"https://analyticsindiamag.com/ai-news-updates/coralogix-introduces-olly-an-advanced-ai-powered-observability-agent-in-india/",
	// 	"https://minutemirror.com.pk/lahore-high-court-halts-transfer-of-brazilian-monkeys-to-lahore-zoo-406544/",
	// 	"https://www.klaviyo.com/blog/introducing-klaviyo-partner-portal",
	// 	"https://www.turkishminute.com/2025/06/25/turkey-relieved-as-iran-israel-truce-reduces-risk-of-regional-turmoil/",
	// 	"https://www.carsandhorsepower.com/featured/exploring-local-old-car-shows-near-you",
	// 	"https://philarchive.org/rec/SCHTCC-30",
	// }
	// beans := ds.QueryBeans(urls)
	// pp.Println(beans)
	// categories := ds.MatchCategories(urls, 3)
	// categories := ds.QueryTags(urls, BEAN_CATEGORIES)
	// pp.Println(categories)
	// sentiments := ds.MatchSentiments(urls, 3)
	// sentiments := ds.QueryTags(urls, BEAN_SENTIMENTS)
	// pp.Println(sentiments)
	// // clusters := ds.MatchClusters(urls, 0.5, 3)
	// // pp.Println(clusters)

	// // gists := ds.QueryTags(urls, BEAN_GISTS)
	// // pp.Println(gists)
	// chatters := ds.QueryChatters(urls)
	// pp.Println(chatters)
	// regions := ds.QueryTags(urls, BEAN_REGIONS)
	// pp.Println(regions)
	// entities := ds.QueryTags(urls, BEAN_ENTITIES)
	// pp.Println(entities)

	urls := []string{
		"https://www.slashgear.com/1896648/lifesaber-emergency-tool-usb-powered-features/",
		"https://issuepay.app",
		"https://jameshard.ing/pilot",
		"https://jobsbyreferral.com/",
		"https://llmapitest.com/",
		"https://htmlrev.com/",
	}

	embeddings := ds.QueryBeanEmbeddings(urls)
	for _, embedding := range embeddings {
		// pp.Println(embedding.URL, embedding.Embedding)

		similars := ds.VectorSearchBeans(embedding.Embedding, 5)
		pp.Println(embedding.URL, datautils.Transform(similars, func(s *EmbeddingData) string {
			return s.URL
		}))
	}

	// pp.Println("CHATTERS", ds.QueryChatters(urls)[:5])
	// pp.Println("AGGREGATES", ds.QueryChatterAggregates(urls))
	// pp.Println("UPDATES", ds.QueryChatterUpdates(urls, 1))
}
