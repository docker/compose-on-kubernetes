package convert

import (
	"reflect"
	"sort"
	"testing"

	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	"github.com/docker/compose-on-kubernetes/internal/stackresources"
	. "github.com/docker/compose-on-kubernetes/internal/test/builders"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
)

func TestToPodWithPlacement(t *testing.T) {
	s := Stack("demo",
		WithService("redis",
			Image("redis:alpine"),
			Deploy(Placement(
				Constraints(
					OperatingSystem("linux", "=="),
					Architecture("amd64", "=="),
					ConstraintHostname("node01", "=="),
					WithMatchLabel("label1", "value1", "=="),
					WithMatchLabel("label2.subpath", "value2", "!="),
				),
			)),
		),
	)
	stack, err := StackToStack(*s, loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.NoError(t, err)
	expectedRequirements := []apiv1.NodeSelectorRequirement{
		{Key: "beta.kubernetes.io/os", Operator: apiv1.NodeSelectorOpIn, Values: []string{"linux"}},
		{Key: "beta.kubernetes.io/arch", Operator: apiv1.NodeSelectorOpIn, Values: []string{"amd64"}},
		{Key: "kubernetes.io/hostname", Operator: apiv1.NodeSelectorOpIn, Values: []string{"node01"}},
		{Key: "label1", Operator: apiv1.NodeSelectorOpIn, Values: []string{"value1"}},
		{Key: "label2.subpath", Operator: apiv1.NodeSelectorOpNotIn, Values: []string{"value2"}},
	}

	requirements := stack.Deployments["redis"].Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions

	sort.Slice(expectedRequirements, func(i, j int) bool { return expectedRequirements[i].Key < expectedRequirements[j].Key })
	sort.Slice(requirements, func(i, j int) bool { return requirements[i].Key < requirements[j].Key })

	assert.EqualValues(t, expectedRequirements, requirements)
}

type keyValue struct {
	key   string
	value string
}

func kv(key, value string) keyValue {
	return keyValue{key: key, value: value}
}

func makeExpectedAffinity(kvs ...keyValue) *apiv1.Affinity {

	var matchExpressions []apiv1.NodeSelectorRequirement
	for _, kv := range kvs {
		matchExpressions = append(
			matchExpressions,
			apiv1.NodeSelectorRequirement{
				Key:      kv.key,
				Operator: apiv1.NodeSelectorOpIn,
				Values:   []string{kv.value},
			},
		)
	}
	return &apiv1.Affinity{
		NodeAffinity: &apiv1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &apiv1.NodeSelector{
				NodeSelectorTerms: []apiv1.NodeSelectorTerm{
					{
						MatchExpressions: matchExpressions,
					},
				},
			},
		},
	}
}

func TestNodeAfinity(t *testing.T) {
	cases := []struct {
		name     string
		source   *latest.Constraints
		expected *apiv1.Affinity
	}{
		{
			name: "nil",
			expected: makeExpectedAffinity(
				kv(kubernetesOs, "linux"),
				kv(kubernetesArch, "amd64"),
			),
		},
		{
			name: "hostname",
			source: &latest.Constraints{
				Hostname: &latest.Constraint{
					Operator: "==",
					Value:    "test",
				},
			},
			expected: makeExpectedAffinity(
				kv(kubernetesHostname, "test"),
				kv(kubernetesOs, "linux"),
				kv(kubernetesArch, "amd64"),
			),
		},
		{
			name: "os",
			source: &latest.Constraints{
				OperatingSystem: &latest.Constraint{
					Operator: "==",
					Value:    "windows",
				},
			},
			expected: makeExpectedAffinity(
				kv(kubernetesOs, "windows"),
				kv(kubernetesArch, "amd64"),
			),
		},
		{
			name: "arch",
			source: &latest.Constraints{
				Architecture: &latest.Constraint{
					Operator: "==",
					Value:    "arm64",
				},
			},
			expected: makeExpectedAffinity(
				kv(kubernetesArch, "arm64"),
				kv(kubernetesOs, "linux"),
			),
		},
		{
			name: "custom-labels",
			source: &latest.Constraints{
				MatchLabels: map[string]latest.Constraint{
					kubernetesArch: {
						Operator: "==",
						Value:    "arm64",
					},
					kubernetesOs: {
						Operator: "==",
						Value:    "windows",
					},
				},
			},

			expected: makeExpectedAffinity(
				kv(kubernetesArch, "arm64"),
				kv(kubernetesOs, "windows"),
			),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result, err := toNodeAffinity(c.source)
			assert.NoError(t, err)
			assert.True(t, nodeAffinityMatch(c.expected, result))
		})
	}
}

func nodeSelectorRequirementsToMap(source []apiv1.NodeSelectorRequirement, result map[string]apiv1.NodeSelectorRequirement) {
	for _, t := range source {
		result[t.Key] = t
	}
}

func nodeAffinityMatch(expected, actual *apiv1.Affinity) bool {
	expectedTerms := expected.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
	actualTerms := actual.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
	expectedExpressions := make(map[string]apiv1.NodeSelectorRequirement)
	expectedFields := make(map[string]apiv1.NodeSelectorRequirement)
	actualExpressions := make(map[string]apiv1.NodeSelectorRequirement)
	actualFields := make(map[string]apiv1.NodeSelectorRequirement)
	for _, v := range expectedTerms {
		nodeSelectorRequirementsToMap(v.MatchExpressions, expectedExpressions)
		nodeSelectorRequirementsToMap(v.MatchFields, expectedFields)
	}
	for _, v := range actualTerms {
		nodeSelectorRequirementsToMap(v.MatchExpressions, actualExpressions)
		nodeSelectorRequirementsToMap(v.MatchFields, actualFields)
	}
	return reflect.DeepEqual(expectedExpressions, actualExpressions) && reflect.DeepEqual(expectedFields, actualFields)
}
