package hashtag

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/logger"
)

// HashtagSuggestion represents predicted hashtags
type HashtagSuggestion struct {
	Suggested      []string       `json:"suggested"`
	Top5           []HashtagScore `json:"top5"`
	InferenceTime  float64        `json:"inference_time_ms"`
	UsedBart       bool           `json:"used_bart"`
}

// HashtagScore represents a hashtag with confidence
type HashtagScore struct {
	Hashtag    string  `json:"hashtag"`
	Confidence float64 `json:"confidence"`
}

// Service provides hashtag prediction and data collection
type Service struct {
	config *config.HashtagConfig
	logger *logger.Logger
	client *Client
	cache  *suggestionCache
}

// NewService creates a new hashtag service
func NewService(cfg *config.HashtagConfig, log *logger.Logger, client *Client) *Service {
	return &Service{
		config: cfg,
		logger: log,
		client: client,
		cache:  newSuggestionCache(time.Duration(cfg.Cache.TTL)*time.Minute, cfg.Cache.MaxSize),
	}
}

// GetSuggestions gets hashtag suggestions for event text
func (hs *Service) GetSuggestions(ctx context.Context, eventText string, useBart bool) (*HashtagSuggestion, error) {
	if eventText == "" {
		return &HashtagSuggestion{
			Suggested: []string{},
			Top5:      []HashtagScore{},
		}, nil
	}

	// Check if service is available
	if hs.client == nil {
		hs.logger.Warn("Hashtag client not initialized, returning empty suggestions")
		return &HashtagSuggestion{
			Suggested: []string{},
			Top5:      []HashtagScore{},
		}, nil
	}

	// Check cache first
	cacheKey := fmt.Sprintf("%s:%v", eventText, useBart)
	if cached, found := hs.cache.get(cacheKey); found {
		hs.logger.Info("Cache hit for hashtag prediction", "event_text", truncate(eventText, 50))
		return cached, nil
	}

	// Get from ML service
	resp, err := hs.client.PredictHashtags(ctx, eventText, useBart, 0.3)
	if err != nil {
		hs.logger.Warn("Failed to get suggestions from hashtag service, returning empty results",
			"error", err,
			"event_text", truncate(eventText, 50))
		// Return empty suggestions instead of error - graceful degradation
		return &HashtagSuggestion{
			Suggested: []string{},
			Top5:      []HashtagScore{},
		}, nil
	}

	// Convert to domain model
	suggestion := &HashtagSuggestion{
		Suggested:     resp.Suggested,
		Top5:          make([]HashtagScore, 0, len(resp.Top_5)),
		InferenceTime: resp.InferenceTimeMs,
		UsedBart:      resp.UsedBart,
	}

	for _, item := range resp.Top_5 {
		suggestion.Top5 = append(suggestion.Top5, HashtagScore{
			Hashtag:    item.Hashtag,
			Confidence: item.Confidence,
		})
	}

	// Cache the result
	hs.cache.set(cacheKey, suggestion)

	return suggestion, nil
}

// GetQuickSuggestions gets fast suggestions (DistilBERT only, no BART)
func (hs *Service) GetQuickSuggestions(ctx context.Context, eventText string) (*HashtagSuggestion, error) {
	return hs.GetSuggestions(ctx, eventText, false)
}

// GetAccurateSuggestions gets accurate suggestions (with BART)
func (hs *Service) GetAccurateSuggestions(ctx context.Context, eventText string) (*HashtagSuggestion, error) {
	return hs.GetSuggestions(ctx, eventText, true)
}

// PredictHashtags is a simple convenience method that returns just the suggested hashtags
// Returns empty array on error instead of failing - graceful degradation
func (hs *Service) PredictHashtags(eventText string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	suggestions, err := hs.GetQuickSuggestions(ctx, eventText)
	if err != nil {
		// Already logged in GetSuggestions
		return []string{}, nil
	}

	return suggestions.Suggested, nil
}

// RecordSelection records user's hashtag selection for incremental learning
// This is async and non-blocking
func (hs *Service) RecordSelection(userID int32, eventText string, selectedHashtags []string) {
	if len(selectedHashtags) == 0 {
		return
	}

	// Run in background
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		timestamp := time.Now().Format(time.RFC3339)
		err := hs.client.CollectData(ctx, userID, eventText, selectedHashtags, timestamp)
		if err != nil {
			hs.logger.Error("Failed to record hashtag selection", "error", err)
		}
	}()
}

// IsServiceAvailable checks if the hashtag service is available
func (hs *Service) IsServiceAvailable() bool {
	return hs.client.IsAvailable()
}

// suggestionCache provides in-memory caching for predictions
type suggestionCache struct {
	mu      sync.RWMutex
	items   map[string]*cacheItem
	ttl     time.Duration
	maxSize int
}

type cacheItem struct {
	value      *HashtagSuggestion
	expiration time.Time
}

func newSuggestionCache(ttl time.Duration, maxSize int) *suggestionCache {
	cache := &suggestionCache{
		items:   make(map[string]*cacheItem),
		ttl:     ttl,
		maxSize: maxSize,
	}

	// Start cleanup goroutine
	go cache.cleanup()

	return cache
}

func (c *suggestionCache) get(key string) (*HashtagSuggestion, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, found := c.items[key]
	if !found {
		return nil, false
	}

	if time.Now().After(item.expiration) {
		return nil, false
	}

	return item.value, true
}

func (c *suggestionCache) set(key string, value *HashtagSuggestion) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest if at max size
	if len(c.items) >= c.maxSize {
		c.evictOldest()
	}

	c.items[key] = &cacheItem{
		value:      value,
		expiration: time.Now().Add(c.ttl),
	}
}

func (c *suggestionCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, item := range c.items {
		if oldestKey == "" || item.expiration.Before(oldestTime) {
			oldestKey = key
			oldestTime = item.expiration
		}
	}

	if oldestKey != "" {
		delete(c.items, oldestKey)
	}
}

func (c *suggestionCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, item := range c.items {
			if now.After(item.expiration) {
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
	}
}

