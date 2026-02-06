// Copyright Contributors to the KubeOpenCode project

package controller

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// TasksTotal is a gauge tracking the number of tasks by namespace and phase.
	TasksTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kubeopencode_tasks_total",
			Help: "Number of tasks by namespace and phase",
		},
		[]string{"namespace", "phase"},
	)

	// TaskDurationSeconds is a histogram tracking task execution duration in seconds.
	TaskDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kubeopencode_task_duration_seconds",
			Help:    "Duration of task execution in seconds",
			Buckets: prometheus.ExponentialBuckets(10, 2, 10), // 10s, 20s, 40s, ... ~2.8h
		},
		[]string{"namespace", "agent"},
	)

	// AgentCapacity is a gauge tracking remaining capacity per agent.
	AgentCapacity = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kubeopencode_agent_capacity",
			Help: "Remaining concurrent task capacity per agent (maxConcurrentTasks - running)",
		},
		[]string{"agent", "namespace"},
	)

	// AgentQueueLength is a gauge tracking the number of queued tasks per agent.
	AgentQueueLength = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kubeopencode_agent_queue_length",
			Help: "Number of queued tasks per agent",
		},
		[]string{"agent", "namespace"},
	)
)

func init() {
	metrics.Registry.MustRegister(
		TasksTotal,
		TaskDurationSeconds,
		AgentCapacity,
		AgentQueueLength,
	)
}
