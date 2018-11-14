package envoy

import (
	"net/http"
	"net/url"

	"github.com/prometheus/common/expfmt"
)

const (
	metricsPath = "/stats/prometheus"

	upstreamResponseMetricName = "envoy_cluster_upstream_rq"
	upstreamRequestMetricName  = "envoy_cluster_upstream_rq_total"
)

func (coll *Collector) collectCounters() (CountersByUpstream, error) {
	counters := make(CountersByUpstream)

	// Collect upstream request metrics from Prometheus-format endpoint
	u := url.URL{
		Scheme: "http",
		Host:   coll.AdminHost,
		Path:   metricsPath,
	}

	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	parser := new(expfmt.TextParser)
	metrics, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		return nil, err
	}

	upstreamRespMetrics := metrics[upstreamResponseMetricName]
	if upstreamRespMetrics == nil {
		return nil, nil
	}

	var upstreamCluster string

	for _, metric := range upstreamRespMetrics.Metric {
		count := metric.GetCounter()
		// Get cluster name first
		for _, label := range metric.GetLabel() {
			if label.GetName() == "envoy_cluster_name" {
				upstreamCluster = label.GetValue()
			}
		}
		c, ok := counters[upstreamCluster]
		if !ok {
			c = new(Counters)
			counters[upstreamCluster] = c
		}

		// Collect response codes by metric now
		for _, label := range metric.GetLabel() {
			if label.GetName() == "envoy_response_code" {
				switch label.GetValue()[0] {
				case '2':
					c.UpstreamResp2xx += count.GetValue()
				case '4':
					c.UpstreamResp4xx += count.GetValue()
				case '5':
					c.UpstreamResp5xx += count.GetValue()
				}
			}
		}
	}

	// Set total request count metric
	upstreamReqMetrics := metrics[upstreamRequestMetricName]
	if upstreamReqMetrics == nil {
		return nil, nil
	}

	for _, metric := range upstreamReqMetrics.Metric {
		for _, label := range metric.GetLabel() {
			if label.GetName() == "envoy_cluster_name" {
				upstreamCluster = label.GetValue()
				counters[upstreamCluster].UpstreamReq = metric.GetCounter().GetValue()
			}
		}
	}

	return counters, nil
}
