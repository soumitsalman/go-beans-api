
INSTALL vss;
LOAD vss;

CREATE TABLE IF NOT EXISTS beans (
    url VARCHAR PRIMARY KEY,
    kind VARCHAR NOT NULL,
    title VARCHAR,
    title_length INTEGER DEFAULT 0,
    content TEXT,
    content_length INTEGER DEFAULT 0,
    summary TEXT,
    summary_length INTEGER DEFAULT 0,
    author VARCHAR,
    source VARCHAR,
    created TIMESTAMP,
    collected TIMESTAMP
);

CREATE TABLE IF NOT EXISTS generated_beans (
    url VARCHAR PRIMARY KEY,
    intro TEXT,
    analysis TEXT[],
    insights TEXT[],
    verdict TEXT,
    predictions TEXT[]
);
CREATE TABLE IF NOT EXISTS bean_embeddings (
    url VARCHAR PRIMARY KEY,
    embedding FLOAT[%d] NOT NULL
);
CREATE TABLE IF NOT EXISTS bean_clusters (
    url VARCHAR NOT NULL,
    tag VARCHAR NOT NULL
);
CREATE TABLE IF NOT EXISTS bean_categories (
    url VARCHAR NOT NULL,
    tag VARCHAR NOT NULL
);
CREATE TABLE IF NOT EXISTS bean_sentiments (
    url VARCHAR NOT NULL,
    tag VARCHAR NOT NULL
);
CREATE TABLE IF NOT EXISTS bean_gists (
    url VARCHAR NOT NULL,
    tag TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS bean_regions (
    url VARCHAR NOT NULL,
    tag VARCHAR NOT NULL
);
CREATE TABLE IF NOT EXISTS bean_entities (
    url VARCHAR NOT NULL,
    tag VARCHAR NOT NULL
);

CREATE TABLE IF NOT EXISTS chatters (
    chatter_url VARCHAR NOT NULL,
    bean_url VARCHAR NOT NULL,
    collected TIMESTAMP NOT NULL,
    source VARCHAR NOT NULL,
    forum VARCHAR,
    likes INTEGER DEFAULT 0,
    comments INTEGER DEFAULT 0,
    subscribers INTEGER DEFAULT 0
);

CREATE VIEW IF NOT EXISTS chatter_aggregates AS
SELECT 
    bean_url as url,
    MAX(collected) as last_collected,
    SUM(likes) as total_likes, 
    SUM(comments) as total_comments, 
    SUM(subscribers) as total_subscribers,
    COUNT(chatter_url) as total_shares
FROM(
    SELECT chatter_url,
        FIRST(bean_url) as bean_url, 
        MAX(collected) as collected, 
        MAX(likes) as likes, 
        MAX(comments) as comments,
        MAX(subscribers) as subscribers
    FROM chatters 
    GROUP BY chatter_url
) 
GROUP BY bean_url;


CREATE TABLE IF NOT EXISTS sources (
    name VARCHAR,
    description TEXT,
    base_url VARCHAR PRIMARY KEY,
    domain_name VARCHAR,
    favicon VARCHAR,
    rss_feed VARCHAR
);

CREATE TABLE IF NOT EXISTS categories AS
SELECT 
    _id as category,
    embedding::FLOAT[%d] as embedding
FROM read_parquet('%s');

CREATE VIEW IF NOT EXISTS category_mappings AS
SELECT 
    url,
    category, 
    array_cosine_distance(b.embedding, c.embedding) as distance
FROM bean_embeddings b CROSS JOIN categories c;

CREATE VIEW IF NOT EXISTS top3_categories AS
SELECT m1.url, m1.category FROM category_mappings m1
WHERE category IN (
    SELECT category FROM category_mappings m2
    WHERE m1.url == m2.url
    ORDER BY m2.distance LIMIT 3
)
ORDER BY m1.url, m1.distance;

CREATE TABLE IF NOT EXISTS sentiments AS
SELECT 
    _id as sentiment,
    embedding::FLOAT[%d] as embedding
FROM read_parquet('%s');

CREATE VIEW IF NOT EXISTS sentiment_mappings AS
SELECT 
    url,
    sentiment, 
    array_cosine_distance(b.embedding, s.embedding) as distance
FROM bean_embeddings b CROSS JOIN sentiments s;

CREATE VIEW IF NOT EXISTS top3_sentiments AS
SELECT url, sentiment FROM sentiment_mappings m1
WHERE sentiment IN (
    SELECT sentiment FROM sentiment_mappings m2
    WHERE m1.url == m2.url
    ORDER BY m2.distance LIMIT 3
)
ORDER BY url, distance;