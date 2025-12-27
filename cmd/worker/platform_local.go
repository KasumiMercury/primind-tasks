//go:build !gcloud

package main

import (
	"context"
	"os"

	"github.com/KasumiMercury/primind-tasks/internal/observability"
	"github.com/KasumiMercury/primind-tasks/internal/observability/logging"
)

func initObservability(ctx context.Context) (*observability.Resources, error) {
	serviceName := os.Getenv("SERVICE_NAME")
	if serviceName == "" {
		serviceName = "primind-tasks-worker"
	}

	env := logging.EnvDev
	if e := os.Getenv("ENV"); e != "" {
		env = logging.Environment(e)
	}

	return observability.Init(ctx, observability.Config{
		ServiceInfo: logging.ServiceInfo{
			Name:     serviceName,
			Version:  Version,
			Revision: "",
		},
		Environment:   env,
		GCPProjectID:  "",
		SamplingRate:  1.0,
		DefaultModule: logging.Module("taskqueue"),
	})
}
