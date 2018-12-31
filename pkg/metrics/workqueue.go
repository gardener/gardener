package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/util/workqueue"
)

var (
	workqueueLabels = []string{"queue"}
	workqueueLength = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "garden_cm_workqueue_items_total",
		Help: "Current count of item in the workqueue.",
	}, workqueueLabels)

	workqueueAdds = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "garden_cm_workqueue_items_adds_total",
		Help: "Count of item additions to a workqueue.",
	}, workqueueLabels)

	workqueueLatency = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name: "garden_cm_workqueue_latency_milliseconds",
		Help: "Time in milliseconds an item remains in the workqueue before it get processed.",
	}, workqueueLabels)

	workqueueDuration = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name: "garden_cm_workqueue_duration_milliseconds",
		Help: "Processing duration in milliseconds of an workqueue item.",
	}, workqueueLabels)

	workqueueRetries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "garden_cm_workqueue_retries_total",
		Help: "Count of item processing retries in a workqueue.",
	}, workqueueLabels)

	workqueueUnfinishedWork = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "garden_cm_workqueue_unfinishedwork_seconds",
		Help: "Unfinished work in seconds.",
	}, workqueueLabels)

	workqueueLongestRunningProcessorMicroseconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "garden_cm_workqueue_longest_running_processor_microseconds",
		Help: "Longest running processor in microseconds.",
	}, workqueueLabels)
)

type workqueueMetricProvider struct{}

func (p workqueueMetricProvider) NewDepthMetric(name string) workqueue.GaugeMetric {
	return workqueueLength.With(prometheus.Labels{"queue": name})
}

func (p workqueueMetricProvider) NewAddsMetric(name string) workqueue.CounterMetric {
	return workqueueAdds.With(prometheus.Labels{"queue": name})
}

func (p workqueueMetricProvider) NewLatencyMetric(name string) workqueue.SummaryMetric {
	return workqueueLatency.With(prometheus.Labels{"queue": name})
}

func (p workqueueMetricProvider) NewWorkDurationMetric(name string) workqueue.SummaryMetric {
	return workqueueDuration.With(prometheus.Labels{"queue": name})
}

func (p workqueueMetricProvider) NewRetriesMetric(name string) workqueue.CounterMetric {
	return workqueueRetries.With(prometheus.Labels{"queue": name})
}

func (p workqueueMetricProvider) NewUnfinishedWorkSecondsMetric(name string) workqueue.SettableGaugeMetric {
	return workqueueUnfinishedWork.With(prometheus.Labels{"queue": name})
}

func (p workqueueMetricProvider) NewLongestRunningProcessorMicrosecondsMetric(name string) workqueue.SettableGaugeMetric {
	return workqueueLongestRunningProcessorMicroseconds.With(prometheus.Labels{"queue": name})
}

// RegisterWorkqueMetrics creates and register a provider for workqueue metrics
// which is used to map the data collected by the workqueues to the proper metrics.
// The provider needs to be registered, before creating workqueues otherwise it wouldn't be noticed.
func RegisterWorkqueMetrics() {
	prometheus.MustRegister(workqueueLength)
	prometheus.MustRegister(workqueueAdds)
	prometheus.MustRegister(workqueueLatency)
	prometheus.MustRegister(workqueueDuration)
	prometheus.MustRegister(workqueueRetries)
	prometheus.MustRegister(workqueueUnfinishedWork)
	prometheus.MustRegister(workqueueLongestRunningProcessorMicroseconds)
	workqueue.SetProvider(workqueueMetricProvider{})
}
