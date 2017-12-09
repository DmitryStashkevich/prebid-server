package logs

import (
	"github.com/prebid/prebid-server/adapters"
	"github.com/prebid/prebid-server/config"
	"github.com/rcrowley/go-metrics"
	"net/http"
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
	//TODO: Replace with switch case for each type of logging
	return setupMetrics(m, e)
}

type LogParams struct {
	ReqType     string
	Request     string
	W           *http.ResponseWriter
	StatusCode  int
	ResponseMsg string
	Logger      *TransactionLogger
}

func (re *LogParams) Write() {
	if re.StatusCode != http.StatusOK {
		(*re.W).WriteHeader(re.StatusCode)
		(*re.W).Write([]byte(re.ResponseMsg))
	}
	(*re.Logger).LogTransaction(re.ReqType, re.Request, re.ResponseMsg, re.StatusCode)
}
