// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheusremotewritereceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

// Factory for prometheusremotewrite
const (
	// Key to invoke this receiver (prometheus_exec)
	typeStr         = "prometheusremotewrite"
	defaultEndpoint = "cloudedge.com"
)

// NewFactory creates a factory for the prometheusremotewrite receiver
func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithMetrics(createMetricsReceiver))
}

// CreateDefaultConfig creates and returns the default configuration for Prometheus Remote Write Receiver.
func createDefaultConfig() configmodels.Receiver {
	return &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: defaultEndpoint,
		},
	}
}

// createMetricsReceiver creates a metrics receiver based on provided Config.
func createMetricsReceiver(ctx context.Context, params component.ReceiverCreateParams,
	cfg configmodels.Receiver, nextConsumer consumer.MetricsConsumer) (component.MetricsReceiver, error) {
	rCfg := cfg.(*Config)
	return NewMetricsReceiver(params.Logger, rCfg, nextConsumer)
}
