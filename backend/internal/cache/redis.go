package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var Client *redis.Client

// Cache TTL constants
const (
	TTLShort   = 5 * time.Minute    // 5 min for frequently changing data
	TTLMedium  = 30 * time.Minute   // 30 min for semi-static data
	TTLLong    = 2 * time.Hour      // 2 hours for stable data
	TTLNoExpire = 0                // No expiration (manual invalidation)
)

const (
	KeyPrefixSearch  = "search"
	KeyPrefixNews    = "news"
	KeyPrefixFeed    = "feed"
	KeyPrefixFeedList = "feedlist"
	KeyPrefixCategories = "categories"
	KeyPrefixCountries  = "countries"
	KeyPrefixStats     = "stats"
	KeyPrefixEntities  = "entities"
	KeyPrefixStatsTop  = "statstop"
)

func Connect(redisURL string) error {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return fmt.Errorf("failed to parse redis URL: %w", err)
	}

	Client = redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err = Client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to ping redis: %w", err)
	}

	return nil
}

func Close() {
	if Client != nil {
		Client.Close()
	}
}

func GetClient() *redis.Client {
	return Client
}

func SearchKey(query, lang string, page, perPage int) string {
	return fmt.Sprintf("%s:%s:%s:%d:%d", KeyPrefixSearch, query, lang, page, perPage)
}

func NewsKey(newsID int64, lang string) string {
	return fmt.Sprintf("%s:%d:%s", KeyPrefixNews, newsID, lang)
}

func FeedKey(feedID int64) string {
	return fmt.Sprintf("%s:%d", KeyPrefixFeed, feedID)
}

func FeedListKey(filters string, page, perPage int) string {
	return fmt.Sprintf("%s:%s:%d:%d", KeyPrefixFeedList, filters, page, perPage)
}

func CategoriesKey() string {
	return KeyPrefixCategories
}

func CountriesKey() string {
	return KeyPrefixCountries
}

func StatsKey() string {
	return KeyPrefixStats
}

func StatsTopKey(typ string) string {
	return fmt.Sprintf("%s:%s", KeyPrefixStatsTop, typ)
}

func EntityKey(entityType, query string, page, perPage int) string {
	return fmt.Sprintf("%s:%s:%s:%d:%d", KeyPrefixEntities, entityType, query, page, perPage)
}

func Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return Client.Set(ctx, key, data, expiration).Err()
}

func Get(ctx context.Context, key string) (string, error) {
	return Client.Get(ctx, key).Result()
}

func Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return Client.Del(ctx, keys...).Err()
}

func DeletePattern(ctx context.Context, pattern string) error {
	iter := Client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		if err := Client.Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}

func InvalidateSearch(ctx context.Context) error {
	return DeletePattern(ctx, KeyPrefixSearch+":*")
}

func InvalidateNews(ctx context.Context, newsID int64) error {
	pattern := fmt.Sprintf("%s:%d:*", KeyPrefixNews, newsID)
	return DeletePattern(ctx, pattern)
}

func InvalidateFeed(ctx context.Context, feedID int64) error {
	return Delete(ctx, FeedKey(feedID))
}

func InvalidateFeedList(ctx context.Context) error {
	return DeletePattern(ctx, KeyPrefixFeedList+":*")
}

func InvalidateCategories(ctx context.Context) error {
	return Delete(ctx, CategoriesKey())
}

func InvalidateCountries(ctx context.Context) error {
	return Delete(ctx, CountriesKey())
}

func InvalidateStats(ctx context.Context) error {
	keys := []string{
		StatsKey(),
		StatsTopKey("categories"),
		StatsTopKey("countries"),
	}
	return Delete(ctx, keys...)
}

func InvalidateEntities(ctx context.Context, entityType string) error {
	pattern := fmt.Sprintf("%s:%s:*", KeyPrefixEntities, entityType)
	return DeletePattern(ctx, pattern)
}

func Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func Exists(ctx context.Context, key string) (bool, error) {
	count, err := Client.Exists(ctx, key).Result()
	return count > 0, err
}
