package main

import (
	"os"

	"github.com/dangnghia1997/go-gin-woocommerce-proxy/handlers"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// WooCommerce credentials
var (
	rdb *redis.Client
)

func main() {

	// Initialize Redis client
	rdb = redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_ADDR"), // Use default Redis address
	})

	r := gin.Default()

	r.Any("/catalog-service/*proxyPath", handlers.ProxyWoocomerce(rdb))
	r.Any("/cms-service/*proxyPath", handlers.ProxyCms(rdb))

	r.Run(":8080") // Run the server on port 8080
}
