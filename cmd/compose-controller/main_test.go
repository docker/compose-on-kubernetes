package main

import (
	"os"
	"testing"

	"github.com/docker/compose-on-kubernetes/api/constants"

	"github.com/docker/cli/opts"
	log "github.com/sirupsen/logrus"
)

func TestImageServer(t *testing.T) {
	if os.Getenv("TEST_COMPOSE_CONTROLLER") != "" {
		options := defaultOptions()
		options.kubeconfig = ""
		interval := constants.DefaultFullSyncInterval
		options.reconciliationInterval = opts.PositiveDurationOpt{DurationOpt: *opts.NewDurationOpt(&interval)}
		options.logLevel = "debug"
		options.defaultServiceType = "LoadBalancer"
		err := start(&options)
		if err != nil {
			log.Errorf("compose-controller fatal error: %s", err)
			t.Fail()
		} else {
			log.Info("compose-controller exited normally")
		}
	} else {
		t.Skip("skipping test: TEST_COMPOSE_CONTROLLER is not set")
	}
}
