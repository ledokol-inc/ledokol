package load

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var usersCountMetric = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "runner_users_running", Help: "Текущее количество работающих пользователей"},
	[]string{"test_name", "scenario_name"})
var successTransactionCountMetric = promauto.NewHistogramVec(prometheus.HistogramOpts{Name: "runner_transaction_success_duration_seconds", Help: "Время выполнения успешных транзакции"},
	[]string{"test_name", "script_name", "step_name"})
var successScenarioCountMetric = promauto.NewHistogramVec(prometheus.HistogramOpts{Name: "runner_scenario_success_duration_seconds", Help: "Время выполнения успешных итераций сценариев"},
	[]string{"test_name", "scenario_name"})
var failedTransactionCountMetric = promauto.NewCounterVec(prometheus.CounterOpts{Name: "runner_transaction_failed_count_total", Help: "Число неуспешных транзакций"},
	[]string{"test_name", "script_name", "step_name", "no_response"})
var failedScenarioCountMetric = promauto.NewCounterVec(prometheus.CounterOpts{Name: "runner_scenario_failed_count_total", Help: "Число неуспешных итераций сценариев"},
	[]string{"test_name", "scenario_name"})
