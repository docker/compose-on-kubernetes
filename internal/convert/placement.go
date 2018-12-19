package convert

import (
	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	"github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
)

const (
	kubernetesOs       = "beta.kubernetes.io/os"
	kubernetesArch     = "beta.kubernetes.io/arch"
	kubernetesHostname = "kubernetes.io/hostname"
)

// node.id	Node ID	node.id == 2ivku8v2gvtg4
// node.hostname	Node hostname	node.hostname != node-2
// node.role	Node role	node.role == manager
// node.labels	user defined node labels	node.labels.security == high
// engine.labels	Docker Engine's labels	engine.labels.operatingsystem == ubuntu 14.04
func toNodeAffinity(constraints *latest.Constraints) (*apiv1.Affinity, error) {
	if constraints == nil {
		constraints = &latest.Constraints{}
	}
	requirements := []apiv1.NodeSelectorRequirement{}
	if constraints.OperatingSystem != nil {
		operator, err := toRequirementOperator(constraints.OperatingSystem.Operator)
		if err != nil {
			return nil, err
		}
		requirements = append(requirements, apiv1.NodeSelectorRequirement{
			Key:      kubernetesOs,
			Operator: operator,
			Values:   []string{constraints.OperatingSystem.Value},
		})
	}
	if constraints.Architecture != nil {
		operator, err := toRequirementOperator(constraints.Architecture.Operator)
		if err != nil {
			return nil, err
		}
		requirements = append(requirements, apiv1.NodeSelectorRequirement{
			Key:      kubernetesArch,
			Operator: operator,
			Values:   []string{constraints.Architecture.Value},
		})
	}
	if constraints.Hostname != nil {
		operator, err := toRequirementOperator(constraints.Hostname.Operator)
		if err != nil {
			return nil, err
		}
		requirements = append(requirements, apiv1.NodeSelectorRequirement{
			Key:      kubernetesHostname,
			Operator: operator,
			Values:   []string{constraints.Hostname.Value},
		})
	}
	if constraints.MatchLabels != nil {
		for key, constraint := range constraints.MatchLabels {
			operator, err := toRequirementOperator(constraint.Operator)
			if err != nil {
				return nil, err
			}
			requirements = append(requirements, apiv1.NodeSelectorRequirement{
				Key:      key,
				Operator: operator,
				Values:   []string{constraint.Value},
			})
		}
	}
	if !hasRequirement(requirements, kubernetesOs) {
		requirements = append(requirements, apiv1.NodeSelectorRequirement{
			Key:      kubernetesOs,
			Operator: apiv1.NodeSelectorOpIn,
			Values:   []string{"linux"},
		})
	}
	if !hasRequirement(requirements, kubernetesArch) {
		requirements = append(requirements, apiv1.NodeSelectorRequirement{
			Key:      kubernetesArch,
			Operator: apiv1.NodeSelectorOpIn,
			Values:   []string{"amd64"},
		})
	}
	return &apiv1.Affinity{
		NodeAffinity: &apiv1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &apiv1.NodeSelector{
				NodeSelectorTerms: []apiv1.NodeSelectorTerm{
					{
						MatchExpressions: requirements,
					},
				},
			},
		},
	}, nil
}

func hasRequirement(requirements []apiv1.NodeSelectorRequirement, key string) bool {
	for _, r := range requirements {
		if r.Key == key {
			return true
		}
	}
	return false
}

func toRequirementOperator(sign string) (apiv1.NodeSelectorOperator, error) {
	switch sign {
	case "==":
		return apiv1.NodeSelectorOpIn, nil
	case "!=":
		return apiv1.NodeSelectorOpNotIn, nil
	case ">":
		return apiv1.NodeSelectorOpGt, nil
	case "<":
		return apiv1.NodeSelectorOpLt, nil
	default:
		return "", errors.Errorf("operator %s not supported", sign)
	}
}
