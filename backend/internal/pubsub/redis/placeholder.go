package redis

import "github.com/redis/go-redis/v9"

// Placeholder for future Redis-backed publishers/subscribers.
type Client struct {
	Raw *redis.Client
}

func New(raw *redis.Client) *Client {
	return &Client{Raw: raw}
}
