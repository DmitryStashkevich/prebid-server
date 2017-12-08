package logging

import (
	"github.com/rcrowley/go-metrics"
	"github.com/prebid/prebid-server/config"
	"github.com/prebid/prebid-server/adapters"
)

type DomainMetrics struct {
	RequestMeter metrics.Meter
}

const (
	TYPE_INFLUXDB = "influxdb"
	TYPE_GRAPHITE = "graphite"
	TYPE_FILE     = "file"
)

type TransactionLogger interface {
	LogTransaction(string, string, string, int)
}

func SetupLogging(m config.Metrics, e map[string]adapters.Adapter) TransactionLogger {
	//TODO: Replace with switch case for each different type of logging
	return setupMetrics(m, e)
}
