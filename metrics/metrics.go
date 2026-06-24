package metrics

import (
	"errors"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Config struct {
	Namespace string `json:"namespace" yaml:"namespace"`
	Subsystem string `json:"subsystem" yaml:"subsystem"`
}

type Metrics struct {
	Registry           *prometheus.Registry
	Requests           *prometheus.CounterVec
	Duration           *prometheus.HistogramVec
	DependencyDuration *prometheus.HistogramVec
	DependencyErrors   *prometheus.CounterVec
}

func New(cfg Config, component string) (*Metrics, error) {
	return NewWithRegistry(cfg, component, prometheus.DefaultRegisterer)
}

func NewWithRegistry(cfg Config, component string, reg prometheus.Registerer) (*Metrics, error) {
	if cfg.Namespace == "" {
		cfg.Namespace = "aisphere"
	}
	if cfg.Subsystem == "" {
		cfg.Subsystem = component
	}
	m := &Metrics{
		Requests:           prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: cfg.Namespace, Subsystem: cfg.Subsystem, Name: "requests_total", Help: "Total Kratos requests."}, []string{"operation", "code"}),
		Duration:           prometheus.NewHistogramVec(prometheus.HistogramOpts{Namespace: cfg.Namespace, Subsystem: cfg.Subsystem, Name: "request_duration_seconds", Help: "Kratos request duration in seconds.", Buckets: prometheus.DefBuckets}, []string{"operation"}),
		DependencyDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Namespace: cfg.Namespace, Subsystem: cfg.Subsystem, Name: "dependency_duration_seconds", Help: "Dependency call duration in seconds.", Buckets: prometheus.DefBuckets}, []string{"dependency", "operation"}),
		DependencyErrors:   prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: cfg.Namespace, Subsystem: cfg.Subsystem, Name: "dependency_errors_total", Help: "Total dependency call errors."}, []string{"dependency", "operation"}),
	}
	if err := register(reg, m.Requests); err != nil {
		return nil, err
	}
	if err := register(reg, m.Duration); err != nil {
		return nil, err
	}
	if err := register(reg, m.DependencyDuration); err != nil {
		return nil, err
	}
	if err := register(reg, m.DependencyErrors); err != nil {
		return nil, err
	}
	return m, nil
}

func register(reg prometheus.Registerer, c prometheus.Collector) error {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	if err := reg.Register(c); err != nil {
		var already prometheus.AlreadyRegisteredError
		if errors.As(err, &already) {
			return nil
		}
		return err
	}
	return nil
}

func (m *Metrics) Observe(operation string, code int, started time.Time) {
	if m == nil {
		return
	}
	if operation == "" {
		operation = "unknown"
	}
	if m.Requests != nil {
		m.Requests.WithLabelValues(operation, strconv.Itoa(code)).Inc()
	}
	if m.Duration != nil {
		m.Duration.WithLabelValues(operation).Observe(time.Since(started).Seconds())
	}
}

func (m *Metrics) ObserveDependency(dependency, operation string, started time.Time, err error) {
	if m == nil {
		return
	}
	if dependency == "" {
		dependency = "unknown"
	}
	if operation == "" {
		operation = "unknown"
	}
	if m.DependencyDuration != nil {
		m.DependencyDuration.WithLabelValues(dependency, operation).Observe(time.Since(started).Seconds())
	}
	if err != nil && m.DependencyErrors != nil {
		m.DependencyErrors.WithLabelValues(dependency, operation).Inc()
	}
}
