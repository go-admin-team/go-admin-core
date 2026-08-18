package redis

import (
	"context"

	goredis "github.com/redis/go-redis/v9"
)

// dial connects using a Redis URL and verifies the server answers, so that a
// bad address fails where it is configured rather than at the first command.
func dial(ctx context.Context, url string) (*goredis.Client, error) {
	opts, err := goredis.ParseURL(url)
	if err != nil {
		return nil, err
	}

	client := goredis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}
