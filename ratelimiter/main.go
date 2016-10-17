// The ratelimiter-go HTTP Demo

package main

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"strconv"
	"time"

	ratelimiter "github.com/teambition/ratelimiter-go"
	redis "gopkg.in/redis.v4"
)

var limiter *ratelimiter.Limiter

// Implements RedisClient for redis.Client
type redisClient struct {
	*redis.Client
}

func (c *redisClient) RateDel(key string) error {
	return c.Del(key).Err()
}
func (c *redisClient) RateEvalSha(sha1 string, keys []string, args ...interface{}) (interface{}, error) {
	return c.EvalSha(sha1, keys, args...).Result()
}
func (c *redisClient) RateScriptLoad(script string) (string, error) {
	return c.ScriptLoad(script).Result()
}

func init() {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	var err error
	limiter, err = ratelimiter.New(&redisClient{client}, ratelimiter.Options{
		Max:      10,
		Duration: time.Minute, // limit to 1000 requests in 1 minute.
	})
	if err != nil {
		panic(err)
	}
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		res, err := limiter.Get(r.URL.Path)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		header := w.Header()
		header.Set("X-Ratelimit-Limit", strconv.FormatInt(int64(res.Total), 10))
		header.Set("X-Ratelimit-Remaining", strconv.FormatInt(int64(res.Remaining), 10))
		header.Set("X-Ratelimit-Reset", strconv.FormatInt(res.Reset.Unix(), 10))

		if res.Remaining >= 0 {
			w.WriteHeader(200)
			fmt.Fprintf(w, "Path: %q\n", html.EscapeString(r.URL.Path))
			fmt.Fprintf(w, "Remaining: %d\n", res.Remaining)
			fmt.Fprintf(w, "Total: %d\n", res.Total)
			fmt.Fprintf(w, "Duration: %v\n", res.Duration)
			fmt.Fprintf(w, "Reset: %v\n", res.Reset)
		} else {
			after := int64(res.Reset.Sub(time.Now())) / 1e9
			header.Set("Retry-After", strconv.FormatInt(after, 10))
			w.WriteHeader(429)
			fmt.Fprintf(w, "Rate limit exceeded, retry in %d seconds.\n", after)
		}
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
