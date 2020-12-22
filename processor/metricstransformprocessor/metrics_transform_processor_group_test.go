package metricstransformprocessor

import (
	"context"
	"regexp"
	"sort"
	"strings"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/collector/translator/internaldata"
	"google.golang.org/protobuf/testing/protocmp"

	"testing"

	"go.uber.org/zap"
)

type metricsGroupingTest struct {
	name       string // test name
	transforms []internalTransform
	in         consumerdata.MetricsData
	out        []consumerdata.MetricsData
}

var (
	groupingTests = []metricsGroupingTest{
		{
			name: "metric_group_by_strict_name",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "foo/metric"},
					Action:              Group,
					GroupResourceLabels: map[string]string{"resource.type": "foo"},
				},
			},
			in: consumerdata.MetricsData{
				Resource: &resourcepb.Resource{
					Labels: map[string]string{
						"original": "label",
					},
				},
				Metrics: []*metricspb.Metric{
					metricBuilder().setName("foo/metric").
						setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
					metricBuilder().setName("bar/metric").
						setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
				},
			},
			out: []consumerdata.MetricsData{
				{
					Resource: &resourcepb.Resource{
						Labels: map[string]string{
							"original": "label",
						},
					},
					Metrics: []*metricspb.Metric{
						metricBuilder().setName("bar/metric").
							setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
					},
				},
				{
					Resource: &resourcepb.Resource{
						Labels: map[string]string{
							"original":      "label",
							"resource.type": "foo",
						},
					},
					Metrics: []*metricspb.Metric{
						metricBuilder().setName("foo/metric").
							setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
					},
				},
			},
		},
		{
			name: "metric_group_regex_multiple_empty_resource",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("^container.(.*)$")},
					Action:              Group,
					GroupResourceLabels: map[string]string{"resource.type": "container"},
				},
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("^k8s.pod.(.*)$")},
					Action:              Group,
					GroupResourceLabels: map[string]string{"resource.type": "k8s.pod"},
				},
			},
			in: consumerdata.MetricsData{
				Metrics: []*metricspb.Metric{
					metricBuilder().setName("container.cpu.utilization").
						setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
					metricBuilder().setName("container.memory.usage").
						setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
					metricBuilder().setName("k8s.pod.cpu.utilization").
						setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
					metricBuilder().setName("k8s.pod.memory.usage").
						setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
				},
			},
			out: []consumerdata.MetricsData{
				{
					Resource: &resourcepb.Resource{
						Labels: map[string]string{
							"resource.type": "container",
						},
					},
					Metrics: []*metricspb.Metric{
						metricBuilder().setName("container.cpu.utilization").
							setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
						metricBuilder().setName("container.memory.usage").
							setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
					},
				},
				{
					Resource: &resourcepb.Resource{
						Labels: map[string]string{
							"resource.type": "k8s.pod",
						},
					},
					Metrics: []*metricspb.Metric{
						metricBuilder().setName("k8s.pod.cpu.utilization").
							setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
						metricBuilder().setName("k8s.pod.memory.usage").
							setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
					},
				},
			},
		},
	}
)

func sortResourceMetricsByResourceType(l []consumerdata.MetricsData) {
	sort.Slice(l, func(i, j int) bool {
		return strings.Compare(
			l[i].Resource.GetLabels()["resource.type"],
			l[j].Resource.GetLabels()["resource.type"]) < 0
	})
}

func sortMetricsByMetricName(m []*metricspb.Metric) {
	sort.Slice(m, func(i, j int) bool {
		return strings.Compare(
			m[i].MetricDescriptor.GetName(),
			m[j].MetricDescriptor.GetName()) < 0
	})
}

func TestMetricsGrouping(t *testing.T) {
	for _, test := range groupingTests {
		t.Run(test.name, func(t *testing.T) {
			next := new(consumertest.MetricsSink)
			p := newMetricsTransformProcessor(zap.NewExample(), test.transforms)

			mtp, err := processorhelper.NewMetricsProcessor(&Config{
				ProcessorSettings: configmodels.ProcessorSettings{
					TypeVal: typeStr,
					NameVal: typeStr,
				},
			}, next, p, processorhelper.WithCapabilities(processorCapabilities))

			require.NoError(t, err)

			caps := mtp.GetCapabilities()
			assert.Equal(t, true, caps.MutatesConsumedData)

			// process
			cErr := mtp.ConsumeMetrics(context.Background(), internaldata.OCToMetrics(test.in))
			assert.NoError(t, cErr)

			// get and check results

			got := next.AllMetrics()
			require.Equal(t, 1, len(got))

			gotMD := internaldata.MetricsToOC(got[0])
			require.Equal(t, len(test.out), len(gotMD))

			sortResourceMetricsByResourceType(gotMD)
			sortResourceMetricsByResourceType(test.out)

			for idx, out := range gotMD {
				if diff := cmp.Diff(test.out[idx].Resource, out.Resource, protocmp.Transform()); diff != "" {
					t.Errorf("Unexpected difference in resource labels:\n%v", diff)
				}

				sortMetricsByMetricName(out.Metrics)
				sortMetricsByMetricName(test.out[idx].Metrics)

				require.Equal(t, len(test.out[idx].Metrics), len(out.Metrics))
				if diff := cmp.Diff(test.out[idx].Metrics, out.Metrics, protocmp.Transform()); diff != "" {
					t.Errorf("Unexpected difference in Metrics:\n%v", diff)
				}
			}

			ctx := context.Background()
			assert.NoError(t, mtp.Shutdown(ctx))

		})
	}
}