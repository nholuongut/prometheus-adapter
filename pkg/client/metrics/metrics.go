/*
Copyright 2017 The Nho Luong DevOps

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package metrics

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	apimetrics "k8s.io/apiserver/pkg/endpoints/metrics"
	"k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"

	"sigs.k8s.io/prometheus-adapter/pkg/client"
)

var (
	// queryLatency is the total latency of any query going through the
	// various endpoints (query, range-query, series).  It includes some deserialization
	// overhead and HTTP overhead.
	queryLatency = metrics.NewHistogramVec(
		&metrics.HistogramOpts{
			Namespace: "prometheus_adapter",
			Subsystem: "prometheus_client",
			Name:      "request_duration_seconds",
			Help:      "Prometheus client query latency in seconds.  Broken down by target prometheus endpoint and target server",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"path", "server"},
	)
)

func MetricsHandler() (http.HandlerFunc, error) {
	registry := metrics.NewKubeRegistry()
	err := registry.Register(queryLatency)
	if err != nil {
		return nil, err
	}
	apimetrics.Register()
	return func(w http.ResponseWriter, req *http.Request) {
		legacyregistry.Handler().ServeHTTP(w, req)
		metrics.HandlerFor(registry, metrics.HandlerOpts{}).ServeHTTP(w, req)
	}, nil
}

// instrumentedClient is a client.GenericAPIClient which instruments calls to Do,
// capturing request latency.
type instrumentedGenericClient struct {
	serverName string
	client     client.GenericAPIClient
}

func (c *instrumentedGenericClient) Do(ctx context.Context, verb, endpoint string, query url.Values) (client.APIResponse, error) {
	startTime := time.Now()
	var err error
	defer func() {
		endTime := time.Now()
		// skip calls where we don't make the actual request
		if err != nil {
			if _, wasAPIErr := err.(*client.Error); !wasAPIErr {
				// TODO: measure API errors by code?
				return
			}
		}
		queryLatency.With(prometheus.Labels{"path": endpoint, "server": c.serverName}).Observe(endTime.Sub(startTime).Seconds())
	}()

	var resp client.APIResponse
	resp, err = c.client.Do(ctx, verb, endpoint, query)
	return resp, err
}

func InstrumentGenericAPIClient(client client.GenericAPIClient, serverName string) client.GenericAPIClient {
	return &instrumentedGenericClient{
		serverName: serverName,
		client:     client,
	}
}
