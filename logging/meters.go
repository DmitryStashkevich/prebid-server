package logging

import (
	"fmt"
	"github.com/prebid/prebid-server/adapters"
	"github.com/rcrowley/go-metrics"
	"sync"
	"time"
	"net"
	"github.com/golang/glog"
	"github.com/prebid/prebid-server/config"
	"github.com/vrischmann/go-metrics-influxdb"
	"github.com/cyberdelia/go-metrics-graphite"
)

type DomainMeterMetrics struct {
	RequestMeter metrics.Meter
}

type AccountMetrics struct {
	RequestMeter      metrics.Meter
	BidsReceivedMeter metrics.Meter
	PriceHistogram    metrics.Histogram
	// store account by adapter metrics. Type is map[PBSBidder.BidderCode]
	AdapterMetrics map[string]*AdapterMetrics
}

type AdapterMetrics struct {
	NoCookieMeter     metrics.Meter
	ErrorMeter        metrics.Meter
	NoBidMeter        metrics.Meter
	TimeoutMeter      metrics.Meter
	RequestMeter      metrics.Meter
	RequestTimer      metrics.Timer
	PriceHistogram    metrics.Histogram
	BidsReceivedMeter metrics.Meter
}

type PBSMetrics struct {
	exchanges            map[string]adapters.Adapter
	metricsRegistry      metrics.Registry
	mRequestMeter        metrics.Meter
	mAppRequestMeter     metrics.Meter
	mNoCookieMeter       metrics.Meter
	mSafariRequestMeter  metrics.Meter
	mSafariNoCookieMeter metrics.Meter
	mErrorMeter          metrics.Meter
	mInvalidMeter        metrics.Meter
	mRequestTimer        metrics.Timer
	mCookieSyncMeter     metrics.Meter

	adapterMetrics map[string]*AdapterMetrics

	accountMetrics        map[string]*AccountMetrics // FIXME -- this seems like an unbounded queue
	accountMetricsRWMutex sync.RWMutex
}

func setupMetrics(settings config.Metrics, exchanges map[string]adapters.Adapter) (*PBSMetrics) {
	return setup(initReporting(settings), exchanges)
}

func initReporting(settings config.Metrics) metrics.Registry {
	var metricsRegistry metrics.Registry
	if settings.Type == TYPE_INFLUXDB {
		metricsRegistry = metrics.NewPrefixedRegistry(settings.Prefix)
		go influxdb.InfluxDB(
			metricsRegistry,                              // metrics registry
			time.Second*time.Duration(settings.Interval), // interval
			settings.Host,                                // the InfluxDB url
			settings.Database,                            // your InfluxDB database
			settings.Username,                            // your InfluxDB user
			settings.Password,                            // your InfluxDB password
		)

	} else if settings.Type == TYPE_GRAPHITE {
		metricsRegistry = metrics.NewPrefixedRegistry("")
		addr, err := net.ResolveTCPAddr("tcp", settings.Host)
		if err == nil {
			go graphite.Graphite(
				metricsRegistry,                              // metrics registry
				time.Second*time.Duration(settings.Interval), // interval
				settings.Prefix,                              // prefix
				addr,                                         // graphite host
			)
		} else {
			glog.Info(err)
		}
	}
	return metricsRegistry
}

func setup(metricsRegistry metrics.Registry, exchanges map[string]adapters.Adapter) *PBSMetrics {
	m := PBSMetrics{}
	m.metricsRegistry = metricsRegistry
	m.exchanges = exchanges
	m.mRequestMeter = metrics.GetOrRegisterMeter("requests", metricsRegistry)
	m.mAppRequestMeter = metrics.GetOrRegisterMeter("app_requests", metricsRegistry)
	m.mNoCookieMeter = metrics.GetOrRegisterMeter("no_cookie_requests", metricsRegistry)
	m.mSafariRequestMeter = metrics.GetOrRegisterMeter("safari_requests", metricsRegistry)
	m.mSafariNoCookieMeter = metrics.GetOrRegisterMeter("safari_no_cookie_requests", metricsRegistry)
	m.mErrorMeter = metrics.GetOrRegisterMeter("error_requests", metricsRegistry)
	m.mInvalidMeter = metrics.GetOrRegisterMeter("invalid_requests", metricsRegistry)
	m.mRequestTimer = metrics.GetOrRegisterTimer("request_time", metricsRegistry)
	m.mCookieSyncMeter = metrics.GetOrRegisterMeter("cookie_sync_requests", metricsRegistry)

	m.accountMetrics = make(map[string]*AccountMetrics)
	m.adapterMetrics = m.makeExchangeMetrics("adapter")
	return &m
}

func (m *PBSMetrics) makeExchangeMetrics(adapterOrAccount string) map[string]*AdapterMetrics {
	var adapterMetrics = make(map[string]*AdapterMetrics)
	for exchange := range m.exchanges {
		a := AdapterMetrics{}
		a.NoCookieMeter = metrics.GetOrRegisterMeter(fmt.Sprintf("%[1]s.%[2]s.no_cookie_requests", adapterOrAccount, exchange), m.metricsRegistry)
		a.ErrorMeter = metrics.GetOrRegisterMeter(fmt.Sprintf("%[1]s.%[2]s.error_requests", adapterOrAccount, exchange), m.metricsRegistry)
		a.RequestMeter = metrics.GetOrRegisterMeter(fmt.Sprintf("%[1]s.%[2]s.requests", adapterOrAccount, exchange), m.metricsRegistry)
		a.NoBidMeter = metrics.GetOrRegisterMeter(fmt.Sprintf("%[1]s.%[2]s.no_bid_requests", adapterOrAccount, exchange), m.metricsRegistry)
		a.TimeoutMeter = metrics.GetOrRegisterMeter(fmt.Sprintf("%[1]s.%[2]s.timeout_requests", adapterOrAccount, exchange), m.metricsRegistry)
		a.RequestTimer = metrics.GetOrRegisterTimer(fmt.Sprintf("%[1]s.%[2]s.request_time", adapterOrAccount, exchange), m.metricsRegistry)
		a.PriceHistogram = metrics.GetOrRegisterHistogram(fmt.Sprintf("%[1]s.%[2]s.prices", adapterOrAccount, exchange), m.metricsRegistry, metrics.NewExpDecaySample(1028, 0.015))
		if adapterOrAccount != "adapter" {
			a.BidsReceivedMeter = metrics.GetOrRegisterMeter(fmt.Sprintf("%[1]s.%[2]s.bids_received", adapterOrAccount, exchange), m.metricsRegistry)
		}

		adapterMetrics[exchange] = &a
	}
	return adapterMetrics
}

func (m *PBSMetrics) GetMyAccountMetrics(id string) *AccountMetrics {
	var am *AccountMetrics
	var ok bool

	m.accountMetricsRWMutex.RLock()
	am, ok = m.accountMetrics[id]
	m.accountMetricsRWMutex.RUnlock()

	if ok {
		return am
	}

	m.accountMetricsRWMutex.Lock()
	am, ok = m.accountMetrics[id]
	if !ok {
		am = &AccountMetrics{}
		am.RequestMeter = metrics.GetOrRegisterMeter(fmt.Sprintf("account.%s.requests", id), m.metricsRegistry)
		am.BidsReceivedMeter = metrics.GetOrRegisterMeter(fmt.Sprintf("account.%s.bids_received", id), m.metricsRegistry)
		am.PriceHistogram = metrics.GetOrRegisterHistogram(fmt.Sprintf("account.%s.prices", id), m.metricsRegistry, metrics.NewExpDecaySample(1028, 0.015))
		am.AdapterMetrics = m.makeExchangeMetrics(fmt.Sprintf("account.%s", id)) //TODO: Is reinitialization necessary
		m.accountMetrics[id] = am
	}
	m.accountMetricsRWMutex.Unlock()

	return am
}

func (m *PBSMetrics) LogTransaction(reqType string, request string, response string, statusCode int) {
	fmt.Println("Request type: ", reqType)
	fmt.Println("\nRequest body: ", request)
	fmt.Println("\nResponse :", response)
	fmt.Println("\nResponse code: ", statusCode, "\n\n")
}

func (m *PBSMetrics) GetMyAdapterMetrics(bidderCode string) *AdapterMetrics {
	return m.adapterMetrics[bidderCode]
}

func (m *PBSMetrics) UpdateRequestTimerSince(start time.Time) {
	m.mRequestTimer.UpdateSince(start)
}

func (m *PBSMetrics) IncRequest(i int64) {
	m.mRequestMeter.Mark(i)
}

func (m *PBSMetrics) IncSafariRequest(i int64) {
	m.mSafariRequestMeter.Mark(i)
}

func (m *PBSMetrics) IncAppRequest(i int64) {
	m.mAppRequestMeter.Mark(i)
}

func (m *PBSMetrics) IncNoCookie(i int64) {
	m.mNoCookieMeter.Mark(i)
}

func (m *PBSMetrics) IncSafariNoCookie(i int64) {
	m.mSafariNoCookieMeter.Mark(i)
}

func (m *PBSMetrics) IncError(i int64) {
	m.mErrorMeter.Mark(i)
}

func (m *PBSMetrics) IncCookieSync(i int64) {
	m.mCookieSyncMeter.Mark(i)
}

func (am *AccountMetrics) GetMyAdapterMetrics(bidderCode string) *AdapterMetrics {
	return am.AdapterMetrics[bidderCode]
}

func (am *AccountMetrics) IncBidsReceived(i int64) {
	am.BidsReceivedMeter.Mark(i)
}

func (am *AccountMetrics) IncRequest(i int64) {
	am.RequestMeter.Mark(i)
}

func (am *AccountMetrics) UpdatePriceHistogram(cpm int64) {
	am.PriceHistogram.Update(cpm)
}

func (am *AdapterMetrics) IncNoBid(i int64) {
	am.NoBidMeter.Mark(i)
}

func (am *AdapterMetrics) IncBidsReceived(i int64) {
	am.BidsReceivedMeter.Mark(i)
}

func (am *AdapterMetrics) UpdateRequestTimerSince(start time.Time) {
	am.RequestTimer.UpdateSince(start)
}

func (am *AdapterMetrics) IncError(i int64) {
	am.ErrorMeter.Mark(i)
}

func (am *AdapterMetrics) IncTimeOut(i int64) {
	am.TimeoutMeter.Mark(i)
}

func (am *AdapterMetrics) UpdatePriceHistogram(cpm int64) {
	am.PriceHistogram.Update(cpm)
}

func (am *AdapterMetrics) IncRequest(i int64) {
	am.RequestMeter.Mark(i)
}
func (am *AdapterMetrics) IncNoCookie(i int64) {
	am.NoCookieMeter.Mark(i)
}
