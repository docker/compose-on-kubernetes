package compose

import (
	"context"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/docker/compose-on-kubernetes/api/constants"
	"github.com/docker/compose-on-kubernetes/install"
	"github.com/docker/compose-on-kubernetes/internal/e2e/cluster"
	"github.com/docker/compose-on-kubernetes/internal/e2e/wait"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

// Install installs the compose api extension
func Install(config *rest.Config, ns, tag, pullSecret string) (func(), error) {
	_, deleteNs, err := cluster.CreateNamespace(config, config, ns)
	if err != nil {
		return nil, err
	}

	err = install.Uninstall(config, ns, false)
	if err != nil {
		return nil, err
	}

	err = install.WaitNPods(config, ns, 0, install.TimeoutDefault)
	if err != nil {
		return nil, err
	}

	err = install.Unsafe(context.Background(), config, install.UnsafeOptions{
		OptionsCommon: install.OptionsCommon{
			Namespace:              ns,
			Tag:                    tag,
			PullSecret:             pullSecret,
			ReconciliationInterval: constants.DefaultFullSyncInterval,
			HealthzCheckPort:       8080,
		},
		Coverage: true,
	})
	if err != nil {
		return nil, err
	}

	err = install.WaitNPods(config, ns, 2, 2*time.Minute)
	if err != nil {
		return nil, err
	}
	err = wait.For(30, func() (bool, error) {
		return install.IsRunning(config)
	})
	if err != nil {
		return nil, err
	}

	cleanup := func() {
		{
			cmd := exec.Command("./retrieve-coverage")
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				log.Errorf("Unable to retrieve stdout: %s", err)
			} else {
				go io.Copy(os.Stdout, stdout)
			}
			stderr, err := cmd.StderrPipe()
			if err != nil {
				log.Errorf("Unable to retrieve stderr: %s", err)
			} else {
				go io.Copy(os.Stderr, stderr)
			}
			err = cmd.Run()
			if err != nil {
				log.Errorf("Unable to retrieve coverage: %s", err)
			}
		}
		install.Uninstall(config, ns, false)
		deleteNs()
	}

	return cleanup, nil
}
