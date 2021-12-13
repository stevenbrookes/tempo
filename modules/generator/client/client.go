package client

import (
	"flag"
	"io"
	"time"

	"github.com/grafana/dskit/grpcclient"
	ring_client "github.com/grafana/dskit/ring/client"
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/opentracing/opentracing-go"
	"github.com/weaveworks/common/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/grafana/tempo/pkg/tempopb"
)

// Config for an generator client.
type Config struct {
	PoolConfig       ring_client.PoolConfig `yaml:"pool_config,omitempty"`
	RemoteTimeout    time.Duration          `yaml:"remote_timeout,omitempty"`
	GRPCClientConfig grpcclient.Config      `yaml:"grpc_client_config"`
}

type Client struct {
	tempopb.GeneratorClient
	grpc_health_v1.HealthClient
	io.Closer
}

// RegisterFlags registers flags.
func (cfg *Config) RegisterFlags(f *flag.FlagSet) {
	cfg.GRPCClientConfig.RegisterFlagsWithPrefix("generator.client", f)

	f.DurationVar(&cfg.PoolConfig.HealthCheckTimeout, "generator.client.healthcheck-timeout", 1*time.Second, "Timeout for healthcheck rpcs.")
	f.DurationVar(&cfg.PoolConfig.CheckInterval, "generator.client.healthcheck-interval", 15*time.Second, "Interval to healthcheck generators")
	f.BoolVar(&cfg.PoolConfig.HealthCheckEnabled, "generator.client.healthcheck-enabled", true, "Healthcheck generators.")
	f.DurationVar(&cfg.RemoteTimeout, "generator.client.timeout", 5*time.Second, "Timeout for generator client RPCs.")
}

// New returns a new generator client.
func New(addr string, cfg Config) (*Client, error) {
	opts := []grpc.DialOption{
		grpc.WithInsecure(),
	}

	instrumentationOpts, err := cfg.GRPCClientConfig.DialOption(instrumentation())
	if err != nil {
		return nil, err
	}

	opts = append(opts, instrumentationOpts...)
	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{
		GeneratorClient: tempopb.NewGeneratorClient(conn),
		HealthClient:    grpc_health_v1.NewHealthClient(conn),
		Closer:          conn,
	}, nil
}

func instrumentation() ([]grpc.UnaryClientInterceptor, []grpc.StreamClientInterceptor) {
	return []grpc.UnaryClientInterceptor{
			otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer()),
			middleware.ClientUserHeaderInterceptor,
		}, []grpc.StreamClientInterceptor{
			otgrpc.OpenTracingStreamClientInterceptor(opentracing.GlobalTracer()),
			middleware.StreamClientUserHeaderInterceptor,
		}
}
