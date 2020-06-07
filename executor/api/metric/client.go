package metric

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	v1 "github.com/seldonio/seldon-core/operator/apis/machinelearning.seldon.io/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"strings"
	"time"
)

type ClientMetrics struct {
	ClientHandledHistogram *prometheus.HistogramVec
	Predictor              *v1.PredictorSpec
	DeploymentName         string
	ModelName              string
	ImageName              string
	ImageVersion           string
}

func NewClientMetrics(spec *v1.PredictorSpec, deploymentName string, modelName string) *ClientMetrics {
	histogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    ClientRequestsMetricName,
			Help:    "A histogram of latencies for client calls from executor",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10},
		},
		[]string{DeploymentNameMetric, PredictorNameMetric, PredictorVersionMetric, ServiceMetric, ModelNameMetric, ModelImageMetric, ModelVersionMetric, "method", "code"},
	)

	err := prometheus.Register(histogram)
	if err != nil {
		prometheus.Unregister(histogram)
		prometheus.Register(histogram)
	}
	container := v1.GetContainerForPredictiveUnit(spec, modelName)
	imageName := ""
	imageVersion := ""
	if container != nil {
		imageParts := strings.Split(container.Image, ":")
		imageName = imageParts[0]
		if len(imageParts) == 2 {
			imageVersion = imageParts[1]
		}
	}

	return &ClientMetrics{
		ClientHandledHistogram: histogram,
		Predictor:              spec,
		DeploymentName:         deploymentName,
		ModelName:              modelName,
		ImageName:              imageName,
		ImageVersion:           imageVersion,
	}
}

func (m *ClientMetrics) UnaryClientInterceptor() func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		startTime := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		st, _ := status.FromError(err)
		m.ClientHandledHistogram.WithLabelValues(m.DeploymentName, m.Predictor.Name, m.Predictor.Annotations["version"], method, m.ModelName, m.ImageName, m.ImageVersion, "unary", st.Code().String()).Observe(time.Since(startTime).Seconds())
		return err
	}
}
