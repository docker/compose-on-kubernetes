package install

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
	k8sVersion "k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

// Minimum server version required.
var minimumServerVersion = "1.8"

// CheckRequirements fetches the server version and checks it is above the
// minimum required version.
func CheckRequirements(config *rest.Config) error {
	client, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return err
	}

	kubeVersion, err := client.ServerVersion()
	if err != nil {
		return err
	}

	return checkVersion(kubeVersion, minimumServerVersion)
}

func checkVersion(version *k8sVersion.Info, minimumVersion string) error {
	constraint, err := semver.NewConstraint(">= " + minimumVersion)
	if err != nil {
		return err
	}

	versionStr := version.Major + "." + version.Minor
	v, err := semver.NewVersion(strings.TrimSuffix(versionStr, "+"))
	if err != nil {
		return errors.Wrapf(err, "unsupported server version: %s", versionStr)
	}

	if !constraint.Check(v) {
		return fmt.Errorf("unsupported server version: %s < %s", versionStr, minimumVersion)
	}

	return nil
}
