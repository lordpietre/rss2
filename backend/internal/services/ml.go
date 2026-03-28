package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rss2/backend/internal/config"
	"github.com/rss2/backend/internal/models"
)

var (
	cfg *config.Config
)

func Init(c *config.Config) {
	cfg = c
}

type TranslationRequest struct {
	SourceLang string   `json:"source_lang"`
	TargetLang string   `json:"target_lang"`
	Texts      []string `json:"texts"`
}

type TranslationResponse struct {
	Translations []string `json:"translations"`
}

func Translate(ctx context.Context, sourceLang, targetLang string, texts []string) ([]string, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	reqBody := TranslationRequest{
		SourceLang: sourceLang,
		TargetLang: targetLang,
		Texts:      texts,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Post(cfg.TranslationURL+"/translate", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("translation request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("translation service returned status %d", resp.StatusCode)
	}

	var result TranslationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Translations, nil
}

type EmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type EmbeddingResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
}

func GetEmbeddings(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	reqBody := EmbeddingRequest{
		Model: "mxbai-embed-large",
		Input: texts,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}
	resp, err := httpClient.Post(cfg.OllamaURL+"/api/embeddings", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embeddings request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embeddings service returned status %d", resp.StatusCode)
	}

	var result EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Embeddings, nil
}

type NERRequest struct {
	Text string `json:"text"`
}

type NERResponse struct {
	Entities []Entity `json:"entities"`
}

type Entity struct {
	Text  string `json:"text"`
	Label string `json:"label"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

func ExtractEntities(ctx context.Context, text string) ([]Entity, error) {
	reqBody := NERRequest{Text: text}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Post(cfg.SpacyURL+"/ner", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("NER request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NER service returned status %d", resp.StatusCode)
	}

	var result NERResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Entities, nil
}

func SemanticSearch(ctx context.Context, query, lang string, page, perPage int) (*models.NewsListResponse, error) {
	embeddings, err := GetEmbeddings(ctx, []string{query})
	if err != nil {
		return nil, err
	}

	if len(embeddings) == 0 {
		return &models.NewsListResponse{}, nil
	}

	return &models.NewsListResponse{
		News:       []models.NewsWithTranslations{},
		Total:      0,
		Page:       page,
		PerPage:    perPage,
		TotalPages: 0,
	}, nil
}
