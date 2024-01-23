package picolytics

import (
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
)

type Metrics struct {
	buildInfo        *prometheus.GaugeVec
	queueUtilization prometheus.Gauge
	queueSize        prometheus.Gauge
	ingestedEvents   *prometheus.CounterVec
	ingestLatency    *prometheus.HistogramVec
	workerLatency    *prometheus.HistogramVec
	eventErrors      *prometheus.CounterVec
	rateLimiterDrops prometheus.Counter

	loadOne      prometheus.Gauge
	loadFive     prometheus.Gauge
	loadFifteen  prometheus.Gauge
	memAvailable prometheus.Gauge
	memTotal     prometheus.Gauge
	memUsed      prometheus.Gauge
	cpuUsedPct   prometheus.Gauge
}

func setupMetrics(queueSize float64, gitCommit, gitBranch, appVersion string) *Metrics {
	m := Metrics{}
	m.buildInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "picolytics",
		Name:      "build_info",
		Help:      "Build and version info.",
	}, []string{"os", "arch", "branch", "goversion", "commit", "version"})
	m.queueUtilization = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "picolytics",
		Name:      "queue_utilization",
		Help:      "Number of events in queue.",
	})
	m.queueSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "picolytics",
		Name:      "queue_size",
		Help:      "Size of event queue.",
	})
	m.ingestedEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "picolytics",
		Name:      "ingested_events",
		Help:      "Number of ingested events by domain.",
	}, []string{"domain"})
	m.ingestLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "picolytics",
		Name:      "ingest_latency",
		Help:      "Latency of ingested event processing (POST->DB) by domain.",
		Buckets:   []float64{.25, .5, .75, 1, 1.25, 1.5, 2, 3, 5, 8, 11}, // buckets in seconds
	}, []string{"domain"})
	m.workerLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "picolytics",
		Name:      "worker_latency",
		Help:      "Latency of worker event processing (Queue->DB) by domain.",
		Buckets:   []float64{.25, .5, .75, 1, 1.25, 1.5, 2, 3, 5, 8, 11}, // buckets in seconds
	}, []string{"domain"})
	m.eventErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "picolytics",
		Name:      "event_errors",
		Help:      "Number of event errors by kind.",
	}, []string{"kind"})
	m.rateLimiterDrops = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "picolytics",
		Name:      "rate_limiter_drops",
		Help:      "Number of dropped connections due to rate limits.",
	})

	m.loadOne = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "picolytics",
		Name:      "host_load_one",
		Help:      "Host load average over the last minute.",
	})
	m.loadFive = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "picolytics",
		Name:      "host_load_five",
		Help:      "Host load average over the last five minutes.",
	})
	m.loadFifteen = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "picolytics",
		Name:      "host_load_fifteen",
		Help:      "Host load average over the last fifteen minutes.",
	})
	m.memAvailable = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "picolytics",
		Name:      "host_mem_available",
		Help:      "Host amount of available memory.",
	})
	m.memUsed = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "picolytics",
		Name:      "host_mem_used",
		Help:      "Host amount of used memory.",
	})
	m.memTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "picolytics",
		Name:      "host_mem_total",
		Help:      "Host amount of total memory.",
	})
	m.cpuUsedPct = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "picolytics",
		Name:      "host_cpu_idle_pct",
		Help:      "Host percentage of idle CPU.",
	})

	m.buildInfo.With(prometheus.Labels{
		"goversion": runtime.Version(),
		"os":        runtime.GOOS,
		"arch":      runtime.GOARCH,
		"branch":    gitBranch,
		"commit":    gitCommit,
		"version":   appVersion,
	}).Set(1)
	m.queueSize.Set(queueSize)
	// zero values so the metrics show up even if no errors have occurred
	m.eventErrors.WithLabelValues("save").Add(0)
	m.eventErrors.WithLabelValues("parse").Add(0)
	m.eventErrors.WithLabelValues("enrich").Add(0)
	m.eventErrors.WithLabelValues("enqueue").Add(0)
	m.eventErrors.WithLabelValues("queue_full").Add(0)
	return &m
}

func startMetrics(m *Metrics, disableHostMetrics bool) {
	prometheus.MustRegister(
		m.queueUtilization,
		m.buildInfo,
		m.queueSize,
		m.ingestedEvents,
		m.ingestLatency,
		m.eventErrors,
		m.rateLimiterDrops,
	)

	if !disableHostMetrics {
		prometheus.MustRegister(
			m.cpuUsedPct,
			m.loadFifteen,
			m.loadFive,
			m.loadOne,
			m.memAvailable,
			m.memTotal,
			m.memUsed,
		)
		go func() {
			for {
				memStats, err := mem.VirtualMemory()
				if err == nil {
					m.memTotal.Set(float64(memStats.Total))
					m.memAvailable.Set(float64(memStats.Available))
					m.memUsed.Set(float64(memStats.Used))
				}
				loadStats, err := load.Avg()
				if err == nil {
					m.loadOne.Set(loadStats.Load1)
					m.loadFive.Set(loadStats.Load5)
					m.loadFifteen.Set(loadStats.Load15)
				}
				cpuPct, err := cpu.Percent(0, false)
				if err == nil {
					m.cpuUsedPct.Set(cpuPct[0])
				}
				time.Sleep(5 * time.Second)
			}
		}()
	}
}

func stopMetrics(m *Metrics, disableHostMetrics bool) {
	prometheus.Unregister(m.queueUtilization)
	prometheus.Unregister(m.buildInfo)
	prometheus.Unregister(m.queueSize)
	prometheus.Unregister(m.ingestedEvents)
	prometheus.Unregister(m.ingestLatency)
	prometheus.Unregister(m.eventErrors)
	prometheus.Unregister(m.rateLimiterDrops)

	if !disableHostMetrics {
		prometheus.Unregister(m.cpuUsedPct)
		prometheus.Unregister(m.loadFifteen)
		prometheus.Unregister(m.loadFive)
		prometheus.Unregister(m.loadOne)
		prometheus.Unregister(m.memAvailable)
		prometheus.Unregister(m.memTotal)
		prometheus.Unregister(m.memUsed)
	}
}
