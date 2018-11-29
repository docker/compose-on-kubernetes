package main

import (
	"os"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func TestImageServer(t *testing.T) {
	if os.Getenv("TEST_API_SERVER") == "" {
		t.Skip("skipping test: TEST_API_SERVER is not set")
	}
	err := start(func(cmd *cobra.Command) error {
		cmd.SetArgs([]string{
			"--kubeconfig", "",
			"--authentication-kubeconfig", "",
			"--authorization-kubeconfig", "",
			"--secure-port", "9443",
			"--etcd-servers", "http://127.0.0.1:2379",
			"--service-namespace=e2e",
			"--service-name=compose-api",
		})
		return nil
	})
	if err != nil {
		log.Errorf("compose-controller fatal error: %s", err)
		t.Fail()
	}
}
