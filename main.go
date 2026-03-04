package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	bs "github.com/soumitsalman/gobeansack/beansack"
	r "github.com/soumitsalman/gobeansack/router"
)

const (
	defaultPort         = "8080"
	defaultEmbedModel   = "text-embedding-3-small"
	defaultEmbedBaseURL = "https://api.openai.com"
	embedTimeoutSeconds = 30
)

type openAICompatibleEmbedder struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

type embeddingRequest struct {
	Input string `json:"input"`
	Model string `json:"model"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func (e *openAICompatibleEmbedder) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	payload, err := json.Marshal(embeddingRequest{Input: query, Model: e.model})
	if err != nil {
		return nil, err
	}
	url := strings.TrimRight(e.baseURL, "/") + "/v1/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embedding api returned %s", resp.Status)
	}

	out := embeddingResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Data) == 0 || len(out.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}
	return out.Data[0].Embedding, nil
}

func main() {
	_ = godotenv.Load()
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})

	connStr := strings.TrimSpace(os.Getenv("PG_CONNECTION_STRING"))
	if connStr == "" {
		log.Fatal("PG_CONNECTION_STRING is required")
	}
	db, err := bs.NewPGSack(connStr)
	if err != nil {
		log.WithError(err).Fatal("failed to connect postgres")
	}
	defer db.Close()

	embedder := &openAICompatibleEmbedder{
		baseURL: strings.TrimSpace(getEnvOrDefault("OPENAI_COMPAT_BASE_URL", defaultEmbedBaseURL)),
		apiKey:  strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		model:   strings.TrimSpace(getEnvOrDefault("EMBEDDER_MODEL", defaultEmbedModel)),
		client:  &http.Client{Timeout: embedTimeoutSeconds * time.Second},
	}

	handler := r.InitializeRoutes(r.RouterDeps{
		DB:       db,
		Embedder: embedder,
		APIKeys:  r.ParseAPIKeys(os.Getenv("API_KEYS")),
	})

	port := strings.TrimSpace(getEnvOrDefault("PORT", defaultPort))
	addr := "0.0.0.0:" + port
	log.WithField("addr", addr).Info("starting server")
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.WithError(err).Fatal("server error")
	}
}

func getEnvOrDefault(name, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return fallback
}
