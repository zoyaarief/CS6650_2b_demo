package main

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"time"
)

var (
	requestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "product_api_requests_total",
			Help: "Total number of Product API requests.",
		},
		[]string{"method", "endpoint"},
	)

	errorCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "product_api_errors_total",
			Help: "Total number of non-2xx responses.",
		},
		[]string{"method", "endpoint"},
	)

	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "product_api_request_duration_seconds",
			Help:    "Duration of Product API requests.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)
)

func init() {
	prometheus.MustRegister(requestCount, errorCount, requestDuration)
}

func recordMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		// Prometheus scraping /metrics should not affect API measurements.
		if c.FullPath() == "/metrics" {
			return
		}

		endpoint := c.FullPath()
		if endpoint == "" {
			endpoint = c.Request.URL.Path
		}

		method := c.Request.Method
		status := c.Writer.Status()
		duration := time.Since(start).Seconds()

		requestCount.WithLabelValues(method, endpoint).Inc()

		errors := errorCount.WithLabelValues(method, endpoint)
		errors.Add(0) // Makes zero errors visible in Prometheus.
		if status < 200 || status >= 300 {
			errors.Inc()
		}

		requestDuration.WithLabelValues(method, endpoint).Observe(duration)
		entry, _ := json.Marshal(map[string]any{
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
			"method":      method,
			"endpoint":    endpoint,
			"status":      status,
			"duration_ms": duration * 1000,
		})

		log.Print(string(entry))
	}
}

type Product struct {
	ProductID    int32  `json:"product_id" binding:"required,min=1"`
	SKU          string `json:"sku" binding:"required,min=1,max=100"`
	Manufacturer string `json:"manufacturer" binding:"required,min=1,max=200"`
	CategoryID   int32  `json:"category_id" binding:"required,min=1"`
	Weight       *int32 `json:"weight" binding:"required,gte=0"`
	SomeOtherID  int32  `json:"some_other_id" binding:"required,min=1"`
}

type APIError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

type ProductStore struct {
	mu       sync.RWMutex
	products map[int32]Product
}

func respondError(c *gin.Context, status int, code, message, details string) {
	c.JSON(status, APIError{Error: code, Message: message, Details: details})
}

func parseProductID(value string) (int32, error) {
	id, err := strconv.ParseInt(value, 10, 32)
	if err != nil || id < 1 {
		return 0, fmt.Errorf("productId must be a positive 32-bit integer")
	}
	return int32(id), nil
}

// GET /products/:productId
func (s *ProductStore) getProduct(c *gin.Context) {
	id, err := parseProductID(c.Param("productId"))
	if err != nil {
		respondError(c, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found", err.Error())
		return
	}

	s.mu.RLock()
	product, exists := s.products[id]
	s.mu.RUnlock()

	if !exists {
		respondError(c, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found", fmt.Sprintf("Product ID %d", id))
		return
	}

	c.JSON(http.StatusOK, product)
}

// POST /products/:productId/details
func (s *ProductStore) addProductDetails(c *gin.Context) {
	id, err := parseProductID(c.Param("productId"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_INPUT", "Invalid product data", err.Error())
		return
	}

	if c.ContentType() != gin.MIMEJSON {
		respondError(c, http.StatusBadRequest, "INVALID_INPUT", "Request body must be application/json", "")
		return
	}

	var product Product
	if err := c.ShouldBindJSON(&product); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_INPUT", "Invalid product data", err.Error())
		return
	}

	if product.ProductID != id {
		respondError(c, http.StatusBadRequest, "INVALID_INPUT", "product_id must match the URL productId", "")
		return
	}

	s.mu.Lock()
	if _, exists := s.products[id]; !exists {
		s.mu.Unlock()
		respondError(c, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found", fmt.Sprintf("Product ID %d", id))
		return
	}
	s.products[id] = product
	s.mu.Unlock()

	c.Status(http.StatusNoContent)
}

func main() {
	log.SetFlags(0)
	zeroWeight := int32(0)
	store := &ProductStore{
		products: map[int32]Product{
			12345: {
				ProductID:    12345,
				SKU:          "ABC-123-XYZ",
				Manufacturer: "Acme Corporation",
				CategoryID:   456,
				Weight:       &zeroWeight,
				SomeOtherID:  789,
			},
		},
	}

	router := gin.New()
	router.Use(recordMetrics())
	router.Use(gin.CustomRecovery(func(c *gin.Context, recovered any) {
		respondError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", "")
		c.Abort()
	}))

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Temporary middleware used only to test the 500 response.
	// router.Use(func(c *gin.Context) {
	// 	if c.GetHeader("X-Test-Error") == "true" {
	// 		panic("forced test error")
	// 	}
	// 	c.Next()
	// })

	v1 := router.Group("/v1")
	v1.GET("/products/:productId", store.getProduct)
	v1.POST("/products/:productId/details", store.addProductDetails)

	if err := router.Run(":8080"); err != nil {
		panic(err)
	}
}
