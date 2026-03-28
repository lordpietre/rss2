package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var Client *redis.Client

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
	return fmt.Sprintf("search:%s:%s:%d:%d", query, lang, page, perPage)
}

func NewsKey(newsID int64, lang string) string {
	return fmt.Sprintf("news:%d:%s", newsID, lang)
}

func FeedKey(feedID int64) string {
	return fmt.Sprintf("feed:%d", feedID)
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

func Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
