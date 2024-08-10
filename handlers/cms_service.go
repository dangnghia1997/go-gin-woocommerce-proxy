package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

var WpApiUrl = os.Getenv("WP_API_URL")
var WpAppUserName = os.Getenv("WP_APP_USERNAME")
var WpAppPasword = os.Getenv("WP_APP_PASSWORD")

// jsonPath = "/wp/v2"
func ProxyCms(rdb *redis.Client, jsonPath string) func(*gin.Context) {
	return func(c *gin.Context) {
		ctx := context.Background()
		proxyPath := c.Param("proxyPath")
		queryString := c.Request.URL.RawQuery
		targetURL := WpApiUrl + jsonPath + proxyPath

		// Append query string if present
		if queryString != "" {
			targetURL += "?" + queryString
		}

		// Check Redis cache
		cacheKey := targetURL
		cachedResponse, err := rdb.Get(ctx, cacheKey).Result()
		if err == redis.Nil {
			fmt.Println("MISS:", targetURL)
			// Cache miss, call WP CMS API
			client := &http.Client{}
			req, err := http.NewRequest(c.Request.Method, targetURL, c.Request.Body)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			// Add WooCommerce authentication
			auth := base64.StdEncoding.EncodeToString([]byte(WpAppUserName + ":" + WpAppPasword))
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

			// Prepare headers to cache
			headers := make(map[string]string)
			for k, v := range resp.Header {
				headers[k] = strings.Join(v, ",")
			}

			// Serialize headers and body
			cachedResp := CachedResponse{
				Headers: headers,
				Body:    string(body),
			}
			cachedRespData, err := json.Marshal(cachedResp)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// Store response in Redis cache
			err = rdb.Set(ctx, cacheKey, cachedRespData, 10*time.Minute).Err()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// Copy headers from the response
			for k, v := range resp.Header {
				c.Writer.Header().Add(k, strings.Join(v, ","))
			}

			c.Status(resp.StatusCode)
			c.Writer.Write(body)
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		} else {
			fmt.Println("HIT:", targetURL)
			// Cache hit, return cached response
			var cachedResp CachedResponse
			err = json.Unmarshal([]byte(cachedResponse), &cachedResp)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// Copy headers from the cached response
			for k, v := range cachedResp.Headers {
				c.Writer.Header().Add(k, v)
			}

			c.Status(http.StatusOK)
			c.Writer.Write([]byte(cachedResp.Body))
		}
	}
}
