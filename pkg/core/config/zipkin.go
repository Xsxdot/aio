package config

import (
	"github.com/openzipkin/zipkin-go"
	"github.com/openzipkin/zipkin-go/reporter/http"
)

type ZipkinConfig struct {
	Url string `yaml:"url"`
}

func InitZipkin(zipkinConfig ZipkinConfig, appName, host string) *zipkin.Tracer {
	// create a reporter to be used by the tracer
	reporter := http.NewReporter(zipkinConfig.Url)
	// set-up the local endpoint for our service
	endpoint, _ := zipkin.NewEndpoint(appName, host)
	// set-up our sampling strategy
	sampler := zipkin.NewModuloSampler(1)
	// initialize the tracer
	tracer, _ := zipkin.NewTracer(
		reporter,
		zipkin.WithLocalEndpoint(endpoint),
		zipkin.WithSampler(sampler),
	)
	return tracer
}
