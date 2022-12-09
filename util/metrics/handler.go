package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/sirupsen/logrus"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func HttpMetricsCheck(addr string, stop <-chan struct{}) {
	requestDurations := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "A histogram of the HTTP request durations in seconds.",
		Buckets: prometheus.ExponentialBuckets(0.1, 1.5, 5),
	})

	// Create non-global registry.
	registry := prometheus.NewRegistry()

	// Add go runtime metrics and process collectors.
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		requestDurations,
	)

	// go func() {
	// 	for {
	// 		// Record fictional latency. 自定义指标案列
	// 		now := time.Now()
	// 		requestDurations.(prometheus.ExemplarObserver).ObserveWithExemplar(
	// 			time.Since(now).Seconds(), prometheus.Labels{"dummyID": fmt.Sprint(rand.Intn(100000))},
	// 		)
	// 		time.Sleep(600 * time.Millisecond)
	// 	}
	// }()

	// Expose /metrics HTTP endpoint using the created custom registry.
	http.Handle(
		"/metrics", promhttp.HandlerFor(
			registry,
			promhttp.HandlerOpts{
				EnableOpenMetrics: true,
			}),
	)

	server := &http.Server{Addr: addr}
	go func() {
		// To test: curl -H 'Accept: application/openmetrics-text' localhost:23333/metrics
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Errorf("[promethes] metrics server close with err: %+v", err)
		}
	}()
	<-stop
	server.SetKeepAlivesEnabled(false)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logrus.Errorf("[promethes] metrics server stop with err: %+v", err)
	}
}
