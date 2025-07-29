package main

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/k0kubun/pp"
	datautils "github.com/soumitsalman/data-utils"
)

const (
	DEFAULT_DB_PATH     = ""
	DEFAULT_VECTOR_DIM  = 384
	DEFAULT_CLUSTER_EPS = 0.43
)

func flattenTags(url string, data []string) []TagData {
	var tags []TagData = make([]TagData, 0, len(data))
	for _, tag := range data {
		tags = append(tags, TagData{URL: url, Tag: tag})
	}
	return tags
}

func prepareTags(beans []Bean) map[string][]TagData {
	// rough initialization
	results := map[string][]TagData{
		"categories": make([]TagData, 0, 3*len(beans)),
		"sentiments": make([]TagData, 0, 3*len(beans)),
		"regions":    make([]TagData, 0, 3*len(beans)),
		"entities":   make([]TagData, 0, 3*len(beans)),
		"gist":       make([]TagData, 0, len(beans)),
	}
	for _, bean := range beans {
		results["categories"] = append(results["categories"], flattenTags(bean.URL, bean.Categories)...)
		results["sentiments"] = append(results["sentiments"], flattenTags(bean.URL, bean.Sentiments)...)
		results["regions"] = append(results["regions"], flattenTags(bean.URL, bean.Regions)...)
		results["entities"] = append(results["entities"], flattenTags(bean.URL, bean.Entities)...)
		results["gist"] = append(results["gist"], TagData{URL: bean.URL, Tag: bean.Gist})
	}
	return results
}

func testQueryDistinctValues(ds *Ducksack) {
	pp.Println("REGIONS", ds.DistinctRegions()[:5])
	pp.Println("ENTITIES", ds.DistinctEntities()[:5])
	pp.Println("SOURCES", ds.DistinctSources()[:5])
	pp.Println("CATEGORIES", ds.DistinctCategories()[:5])
}

func testQueryChatterStats(ds *Ducksack) {
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

func testQueryBeanExtensions(ds *Ducksack) {
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

func testVectorSearch(ds *Ducksack) {
	query_emb := Float32Array{
		-0.05782698094844818,
		0.012258837930858135,
		-0.016731202602386475,
		-0.029066722840070724,
		0.04909760132431984,
		0.06186132878065109,
		-0.030351659283041954,
		-0.039454177021980286,
		0.008438210003077984,
		-0.025544442236423492,
		-0.0046130395494401455,
		-0.06961511075496674,
		0.001061786781065166,
		0.037250958383083344,
		0.0031552144791930914,
		0.013171163387596607,
		0.021631045266985893,
		-0.06068258360028267,
		-0.027832819148898125,
		0.0479525551199913,
		0.0891757607460022,
		-0.10267972201108932,
		-0.002737079281359911,
		-0.02839360386133194,
		-0.005528009030967951,
		0.01290945429354906,
		-0.04702915623784065,
		-0.02643556334078312,
		-0.021291635930538177,
		-0.20257152616977692,
		-0.010272135958075523,
		-0.08451215922832489,
		-0.0022792997770011425,
		-0.017279988154768944,
		-0.06730510294437408,
		-0.03289305418729782,
		0.0075003523379564285,
		-0.05605108663439751,
		-0.039814408868551254,
		0.03491684049367905,
		-0.00984497182071209,
		0.008044461719691753,
		-0.0050437129102647305,
		-0.021904917433857918,
		-0.05186145007610321,
		-0.04686053842306137,
		-0.014844674617052078,
		0.01849628984928131,
		-0.010572269558906555,
		0.010804376564919949,
		0.05018135905265808,
		-0.012686269357800484,
		0.01908104680478573,
		-0.03050534427165985,
		-0.0008797564078122377,
		0.060477688908576965,
		0.011422819457948208,
		0.027070069685578346,
		0.03026929870247841,
		-0.03873535618185997,
		0.020079059526324272,
		-0.019260887056589127,
		-0.21033449470996857,
		-0.011735864914953709,
		0.006064576096832752,
		0.0017116563394665718,
		0.006534137297421694,
		0.0021597673185169697,
		0.004746844060719013,
		-0.004467836115509272,
		0.018482616171240807,
		-0.013477069325745106,
		0.014407760463654995,
		0.0023255592677742243,
		-0.02370866760611534,
		0.0718897357583046,
		-0.016208169981837273,
		0.030638467520475388,
		-0.012739519588649273,
		-0.02448415756225586,
		0.05463431403040886,
		-0.020046008750796318,
		-0.002826308598741889,
		-0.007779401261359453,
		-0.044537704437971115,
		0.006009906996041536,
		0.050236500799655914,
		-0.030004875734448433,
		0.017095770686864853,
		-0.01076648011803627,
		-0.0680837631225586,
		-0.051064472645521164,
		-0.0018752365140244365,
		0.02967369742691517,
		-0.05715152993798256,
		-0.035160887986421585,
		0.05469316616654396,
		0.012209818698465824,
		-0.06775996088981628,
		0.28550851345062256,
		0.03471823409199715,
		0.0676717683672905,
		0.05600808560848236,
		-0.045770496129989624,
		-0.008957247249782085,
		-0.009997927583754063,
		-0.010713827796280384,
		0.0005844110855832696,
		0.00356445275247097,
		-0.08926264196634293,
		-0.0005041568656452,
		0.0008613422978669405,
		0.012860007584095001,
		0.027769170701503754,
		0.05613572522997856,
		0.05329415202140808,
		-0.005405510775744915,
		0.030541112646460533,
		-0.006897672079503536,
		0.027818717062473297,
		-0.025592928752303123,
		0.0706404522061348,
		0.07554732263088226,
		0.03185072913765907,
		-0.0024710206780582666,
		0.05119210109114647,
		0.044166482985019684,
		0.06259910017251968,
		-0.02049250714480877,
		0.030628342181444168,
		0.016032913699746132,
		0.01885686069726944,
		-0.02756238542497158,
		0.014548520557582378,
		0.033993955701589584,
		0.006051572505384684,
		0.04207294434309006,
		0.00497390516102314,
		0.02226073667407036,
		0.013709018006920815,
		0.05408424884080887,
		0.03952848166227341,
		0.016977393999695778,
		-0.06649594008922577,
		0.003468258073553443,
		0.042586274445056915,
		-0.0031209378503262997,
		-0.01639457419514656,
		-0.06867604702711105,
		-0.00914774090051651,
		0.015532189980149269,
		0.07281976193189621,
		0.05162084475159645,
		-0.04660652205348015,
		0.08143030107021332,
		0.028807038441300392,
		0.04202795401215553,
		-0.0027100294828414917,
		-0.02466749958693981,
		0.002444157376885414,
		-0.044899534434080124,
		0.012032847851514816,
		-0.05620668828487396,
		0.0930124893784523,
		0.07067516446113586,
		-0.08695631474256516,
		-0.03055228851735592,
		0.023594850674271584,
		-0.004997700918465853,
		-0.006433677859604359,
		0.010148582980036736,
		0.07033155858516693,
		0.015594701282680035,
		-0.006426211446523666,
		0.10823539644479752,
		-0.039923299103975296,
		-0.04066724330186844,
		-0.009347429499030113,
		-0.06146639958024025,
		-0.02222832292318344,
		0.008427256718277931,
		0.028181295841932297,
		0.02296583168208599,
		-0.010990699753165245,
		0.0640539899468422,
		-0.0240690428763628,
		0.00019822659669443965,
		-0.046675246208906174,
		-0.024699941277503967,
		-0.02042154036462307,
		-0.0534159317612648,
		0.09275322407484055,
		0.028236091136932373,
		0.0728689506649971,
		0.047365136444568634,
		-0.005697838496416807,
		-0.07003611326217651,
		-0.04355694353580475,
		0.06717932224273682,
		-0.05206305533647537,
		0.03558908402919769,
		0.03352431207895279,
		-0.044332701712846756,
		0.03852824494242668,
		0.016555974259972572,
		0.008238560520112514,
		-0.04389988258481026,
		0.07608445733785629,
		-0.008913356810808182,
		0.009821810759603977,
		-0.007956397719681263,
		-0.0463133305311203,
		0.06594368070363998,
		-0.015606201253831387,
		-0.056516826152801514,
		0.0867144986987114,
		0.04383605346083641,
		0.0073897079564630985,
		0.006829867605119944,
		0.041037701070308685,
		0.004029841627925634,
		0.0014048655284568667,
		-0.05407027527689934,
		-0.28776106238365173,
		0.0002647813525982201,
		-0.026171401143074036,
		-0.009463725611567497,
		0.0429127998650074,
		-0.026462888345122337,
		0.08026540279388428,
		-0.007218286860734224,
		-0.06390231847763062,
		-0.007690213155001402,
		0.021672135218977928,
		-0.018184075132012367,
		-0.014719660393893719,
		-0.010666515678167343,
		0.021379981189966202,
		0.04016108810901642,
		0.053840503096580505,
		-0.06833255290985107,
		-0.004341346677392721,
		0.025027628988027573,
		-0.0022211973555386066,
		0.07245661318302155,
		0.023802848532795906,
		-0.0009377310634590685,
		0.05996432527899742,
		-0.013052751310169697,
		0.1165713220834732,
		-0.09173078835010529,
		0.011766055598855019,
		-0.04894775524735451,
		0.01664290763437748,
		-0.03894587606191635,
		-0.052885472774505615,
		-0.016306288540363312,
		0.035085529088974,
		0.056923411786556244,
		0.022733695805072784,
		0.0417005680501461,
		-0.01960141770541668,
		-0.028015101328492165,
		-0.05783674865961075,
		0.028085840865969658,
		-0.03156294673681259,
		-0.061249684542417526,
		-0.013418824411928654,
		-0.014729781076312065,
		-0.04247681424021721,
		-0.008108711801469326,
		-0.02082413248717785,
		-0.031127234920859337,
		0.045792918652296066,
		-0.016925465315580368,
		0.03763895854353905,
		-0.00014829968858975917,
		0.10984715074300766,
		-0.023429350927472115,
		-0.09602083265781403,
		0.05814167112112045,
		0.010602191090583801,
		-0.04328335449099541,
		-0.0339193195104599,
		-0.044145602732896805,
		-0.055454451590776443,
		-0.001643496099859476,
		0.010072657838463783,
		-0.03962629660964012,
		-0.012369795702397823,
		0.010365051217377186,
		-0.003846930805593729,
		-0.01987875998020172,
		0.013020801357924938,
		0.06144534796476364,
		0.022647518664598465,
		0.012291674502193928,
		0.06181282177567482,
		-0.023727327585220337,
		-0.007970036007463932,
		0.001768652000464499,
		-0.07940638810396194,
		0.006470066029578447,
		0.005384773947298527,
		-0.02311505191028118,
		0.05790458619594574,
		0.01912003941833973,
		0.06618855893611908,
		0.045415304601192474,
		0.0018891404615715146,
		0.023675356060266495,
		-0.03279435634613037,
		-0.010907863266766071,
		-0.0033687143586575985,
		-0.05965639278292656,
		-0.024388698861002922,
		-0.06574584543704987,
		0.08210533112287521,
		0.042672526091337204,
		-0.30187445878982544,
		0.027669018134474754,
		-0.02971654012799263,
		0.03468979522585869,
		-0.020159417763352394,
		-0.02159169875085354,
		0.034799326211214066,
		0.06167219951748848,
		0.01878558285534382,
		0.032595645636320114,
		-0.02467900700867176,
		0.0281978826969862,
		-0.008921450935304165,
		0.03458806127309799,
		-0.00897459127008915,
		-0.004629888106137514,
		0.07234392315149307,
		-0.047911811619997025,
		0.025473251938819885,
		-0.07601040601730347,
		0.06868407130241394,
		-0.020979875698685646,
		0.21454663574695587,
		-0.012022571638226509,
		-0.021422607824206352,
		0.02288609743118286,
		-0.05466056615114212,
		0.08191382884979248,
		0.04580259323120117,
		0.02009134739637375,
		0.011178066954016685,
		-0.02910388819873333,
		0.025893395766615868,
		-0.020880652591586113,
		0.003648499958217144,
		0.04857511445879936,
		-0.020254723727703094,
		0.00279387179762125,
		0.07122453302145004,
		-0.030391769483685493,
		-0.02479351870715618,
		0.008204258978366852,
		-0.020713889971375465,
		-0.006012571044266224,
		0.06389413774013519,
		-0.039680901914834976,
		-0.013078158721327782,
		-0.0986640453338623,
		0.0032812440767884254,
		-0.01891038939356804,
		-0.04796848073601723,
		0.023867007344961166,
		-0.01313840039074421,
		0.012424834072589874,
		0.026679938659071922,
		0.018602030351758003,
		0.004542315844446421,
		-0.024163633584976196,
		-0.025524074211716652,
		-0.04155180603265762,
		0.05408211797475815,
		-0.05275244638323784,
		-0.04850303381681442,
		-0.036028821021318436,
		0.04311073198914528,
	}

	sources := []string{"techstartups", "techradar"}

	datautils.PrintTable(
		ds.VectorSearchBeans(query_emb, 0.25, NEWS, time.Time{}, nil, nil, nil, sources, 0, 5),
		[]string{"kind", "title", "categories", "entities", "created", "source"},
		func(b *Bean) []string {
			return []string{b.Kind, b.Title, strings.Join(b.Categories, ", "), strings.Join(b.Entities, ", "), b.Created.Format(time.RFC3339), b.Source}
		},
	)
}

func testQueryBeans(ds *Ducksack) {
	categories := []string{"Artificial Intelligence", "Cloud Computing"}
	entities := []string{"ChatGPT", "Elon Musk"}

	for i := int64(0); i < 3; i++ {
		datautils.PrintTable(
			ds.QueryBeans(NEWS, time.Now().AddDate(0, 0, -3), categories, nil, entities, nil, i*5, 6),
			[]string{"kind", "title", "categories", "entities", "created", "source"},
			func(b *Bean) []string {
				return []string{b.Kind, b.Title, strings.Join(b.Categories, ", "), strings.Join(b.Entities, ", "), b.Created.Format(time.RFC3339), b.Source}
			},
		)
	}
}

func testQueryRelated(ds *Ducksack) {
	urls := []string{
		"https://decrypt.co/328057/cloudflare-hits-kill-switch-ai-crawlers-entire-industry-cheers",
		"https://uk.pcmag.com/ai/158866/cloudflare-to-block-ai-crawlers-from-scraping-websites-unless-they-pay",
		"https://www.pcmag.com/news/cloudflare-to-block-ai-crawlers-from-scraping-websites-unless-they-pay",
		"https://www.engadget.com/ai/cloudflare-experiment-will-block-ai-bot-scrapers-unless-they-pay-a-fee-1215233…",
		"https://dailygalaxy.com/2025/07/red-jellyfish-bursts-storm-outer-space/",
	}
	pp.Println(ds.GetRelated(urls))
}

func main() {
	// Load configuration from environment variables
	noerror(godotenv.Load(".env"), "LOAD ENV ERROR")
	dbpath, ok := os.LookupEnv("DB_PATH")
	if !ok {
		dbpath = DEFAULT_DB_PATH
	}
	dim, err := strconv.Atoi(os.Getenv("VECTOR_DIM"))
	if err != nil {
		dim = DEFAULT_VECTOR_DIM
	}
	// Get cluster epsilon from env or use default
	cluster_eps, err := strconv.ParseFloat(os.Getenv("CLUSTER_EPS"), 64)
	if err != nil {
		cluster_eps = DEFAULT_CLUSTER_EPS
	}

	// initialize database if needed
	init, err := os.ReadFile("./factory/init.sql")
	noerror(err, "READ SQL ERROR")
	initsql := string(init)

	ds := NewBeansack(dbpath, initsql, dim, cluster_eps)
	defer ds.Close()

	// hydrateTestDB(ds)
	// testQueryDistinctValues(ds)
	// testQueryChatterStats(ds)
	// testQueryBeanExtensions(ds)
	// testQueryBeans(ds)
	testVectorSearch(ds)
	// testQueryRelated(ds)

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

	// urls := []string{
	// 	"https://www.slashgear.com/1896648/lifesaber-emergency-tool-usb-powered-features/",
	// 	"https://www.wusa9.com/article/news/nation-world/trump-big-bill-may-have-political-cost/507-0c07fc4b-248b-4a3b-96c0-81d75a511228",
	// 	"https://issuepay.app",
	// 	"https://minutemirror.com.pk/lahore-high-court-halts-transfer-of-brazilian-monkeys-to-lahore-zoo-406544/",
	// 	"https://jameshard.ing/pilot",
	// 	"https://jobsbyreferral.com/",
	// 	"https://llmapitest.com/",
	// 	"https://htmlrev.com/",
	// }

	// embeddings := ds.QueryBeanEmbeddings(urls)
	// for _, embedding := range embeddings {
	// 	// pp.Println(embedding.URL, embedding.Embedding)

	// 	similars := ds.VectorSearchBeans(embedding.Embedding, 5)
	// 	pp.Println(embedding.URL, datautils.Transform(similars, func(s *EmbeddingData) string {
	// 		return s.URL
	// 		// res, err := json.MarshalIndent(s, "", "  ")
	// 		// noerror(err)
	// 		// return string(res)
	// 	}))
	// }

	// pp.Println("CHATTERS", ds.QueryChatters(urls)[:5])
	// pp.Println("AGGREGATES", ds.QueryChatterAggregates(urls))
	// pp.Println("UPDATES", ds.QueryChatterUpdates(urls, 1))

	// pp.Println("CATEGORIES", ds.QueryCategories(urls))
	// pp.Println("SENTIMENTS", ds.QuerySentiments(urls))
	// pp.Println("RELATED", ds.QueryClusters(urls))
	// pp.Println("REGIONS", ds.QueryRegions(urls))
	// pp.Println("ENTITIES", ds.QueryEntities(urls))
	// pp.Println("GISTS", ds.QueryGists(urls))

	// start := time.Now()
	// res := ds.QueryBeansWithExtensions(urls)
	// pp.Println("BEAN AGGREGATES", res, time.Since(start))

}
