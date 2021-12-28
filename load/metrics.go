package load

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var usersCountMetric = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "runner_users_running", Help: "Текущее количество работающих пользователей"},
	[]string{"test_id"})
var successTransactionCountMetric = promauto.NewHistogramVec(prometheus.HistogramOpts{Name: "runner_transaction_success_duration_seconds", Help: "Время выполнения транзакции"},
	[]string{"test_id", "scenario_name", "step_name"})
var successScenarioCountMetric = promauto.NewHistogramVec(prometheus.HistogramOpts{Name: "runner_scenario_success_duration_seconds", Help: "Время выполнения сценария"},
	[]string{"test_id", "scenario_name"})
var failedTransactionCountMetric = promauto.NewCounterVec(prometheus.CounterOpts{Name: "runner_transaction_failed_count_total", Help: "Количество неуспешных транзакций"},
	[]string{"test_id", "scenario_name", "step_name", "no_response"})
var failedScenarioCountMetric = promauto.NewCounterVec(prometheus.CounterOpts{Name: "runner_scenario_failed_count_total", Help: "Количество неуспешных сценариев"},
	[]string{"test_id", "scenario_name"})
