package observability

import (
	"database/sql"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "http_request_duration_seconds",
	Help:    "HTTP request duration in seconds.",
	Buckets: prometheus.DefBuckets,
}, []string{"method", "route", "status"})

var requestTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "http_requests_total",
	Help: "Total HTTP requests.",
}, []string{"method", "route", "status"})

var dbPoolStats = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "db_pool_stats",
	Help: "Database connection pool stats.",
}, []string{"stat"})

func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unknown"
		}
		status := strconv.Itoa(c.Writer.Status())
		requestTotal.WithLabelValues(c.Request.Method, route, status).Inc()
		requestDuration.WithLabelValues(c.Request.Method, route, status).Observe(time.Since(start).Seconds())
	}
}

func Handler() gin.HandlerFunc {
	handler := promhttp.Handler()
	return func(c *gin.Context) {
		handler.ServeHTTP(c.Writer, c.Request)
	}
}

func RegisterDBStats(db *sql.DB) {
	if db == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			stats := db.Stats()
			dbPoolStats.WithLabelValues("open_connections").Set(float64(stats.OpenConnections))
			dbPoolStats.WithLabelValues("in_use").Set(float64(stats.InUse))
			dbPoolStats.WithLabelValues("idle").Set(float64(stats.Idle))
			dbPoolStats.WithLabelValues("wait_count").Set(float64(stats.WaitCount))
			dbPoolStats.WithLabelValues("wait_duration_seconds").Set(stats.WaitDuration.Seconds())
		}
	}()
}
