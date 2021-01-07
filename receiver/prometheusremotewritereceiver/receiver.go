package prometheusremotewritereceiver

import (
	"context"
	"errors"
	"fmt"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

var (
	errNilNextMetricsConsumer = errors.New("nil metricsConsumer")
	errEmptyEndpoint          = errors.New("empty endpoint")
)

const (
	defaultServerTimeout = 10
	apiPath = "/"
)

// NewMetricsReceiver creates the PrometheusRemoteWriteReceiver
func NewMetricsReceiver(logger *zap.Logger, config *Config,
	nextConsumer consumer.MetricsConsumer) (component.MetricsReceiver, error) {
	if nextConsumer == nil {
		return nil, errNilNextMetricsConsumer
	}

	if config.Endpoint == "" {
		return nil, errEmptyEndpoint
	}

	r := &PromRemoteWriteReceiver{
		logger:          logger,
		config:          config,
		metricsConsumer: nextConsumer,
		server: &http.Server{
			Addr: config.Endpoint,
			ReadHeaderTimeout: defaultServerTimeout,
			WriteTimeout:      defaultServerTimeout,
		},
	}
	return r, nil
}

type PromRemoteWriteReceiver struct {
	logger *zap.Logger
	config *Config
	metricsConsumer consumer.MetricsConsumer
	server *http.Server
	startOnce sync.Once
	stopOnce  sync.Once
}

func (p *PromRemoteWriteReceiver) Start(ctx context.Context, host component.Host) error {
	var err error
	p.startOnce.Do(func(){
		lis, err := p.config.HTTPServerSettings.ToListener()
		if err != nil {
			return
		}

		mx := mux.NewRouter()
		mx.HandleFunc(apiPath, func(w http.ResponseWriter, request *http.Request) {
			compressed, err := ioutil.ReadAll(request.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			reqBuf, err := snappy.Decode(nil, compressed)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			var req prompb.WriteRequest
			if err := proto.Unmarshal(reqBuf, &req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			for _, ts := range req.Timeseries {
				m := make(model.Metric, len(ts.Labels))
				for _, l := range ts.Labels {
					m[model.LabelName(l.Name)] = model.LabelValue(l.Value)
				}
				p.logger.Info(fmt.Sprint(m))

				for _, s := range ts.Samples {
					p.logger.Info(fmt.Sprintf("%f %d\n", s.Value, s.Timestamp))
				}
			}

		})

		p.server = p.config.HTTPServerSettings.ToServer(mx)
		p.server.ReadHeaderTimeout = defaultServerTimeout
		p.server.WriteTimeout = defaultServerTimeout

		go func() {
			if errHTTP := p.server.Serve(lis); errHTTP != nil {
				host.ReportFatalError(errHTTP)
			}
		}()
	})
	return err
}

func (p *PromRemoteWriteReceiver) Shutdown(ctx context.Context) error {
	panic("implement me")
}
