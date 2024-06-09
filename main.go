package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// WooCommerce credentials
var (
	WooCommerceAPIURL = os.Getenv("WOOCOMMERCE_API_URL")
	ConsumerKey       = os.Getenv("WOOCOMMERCE_CONSUMER_KEY")
	ConsumerSecret    = os.Getenv("WOOCOMMERCE_CONSUMER_SECRET")
	ctx               = context.Background()
	rdb               *redis.Client
)

func main() {

	// Initialize Redis client
	rdb = redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_ADDR"), // Use default Redis address
	})

	r := gin.Default()

	r.Any("/api/*proxyPath", func(c *gin.Context) {
		proxyPath := c.Param("proxyPath")
		queryString := c.Request.URL.RawQuery
		targetURL := WooCommerceAPIURL + proxyPath

		// Append query string if present
		if queryString != "" {
			targetURL += "?" + queryString
		}

		// Check Redis cache
		cacheKey := targetURL
		cachedResponse, err := rdb.Get(ctx, cacheKey).Result()
		if err == redis.Nil {
			fmt.Println("MISS:", targetURL)
			// Cache miss, call WooCommerce API
			client := &http.Client{}
			req, err := http.NewRequest(c.Request.Method, targetURL, c.Request.Body)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// Add WooCommerce authentication
			auth := base64.StdEncoding.EncodeToString([]byte(ConsumerKey + ":" + ConsumerSecret))
			req.Header.Add("Authorization", "Basic "+auth)

			// Copy headers from the incoming request
			for k, v := range c.Request.Header {
				req.Header[k] = v
			}

			resp, err := client.Do(req)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// Store response in Redis cache
			err = rdb.Set(ctx, cacheKey, body, 10*time.Minute).Err()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// Copy headers from the response
			for k, v := range resp.Header {
				c.Writer.Header()[k] = v
			}

			c.Status(resp.StatusCode)
			c.Writer.Write(body)
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		} else {
			fmt.Println("HIT:", targetURL)
			// Cache hit, return cached response
			c.Writer.Header().Set("Content-Type", "application/json")
			c.String(http.StatusOK, cachedResponse)
		}
	})

	r.Run(":8080") // Run the server on port 8080
}
