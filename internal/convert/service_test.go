package convert

import (
	"fmt"
	"strings"
	"testing"

	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	"github.com/docker/compose-on-kubernetes/internal/stackresources"
	. "github.com/docker/compose-on-kubernetes/internal/test/builders"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func services(t *testing.T, stack *latest.Stack, strategy ServiceStrategy) (*apiv1.Service, *apiv1.Service, *apiv1.Service) {
	s, err := StackToStack(*stack, strategy, stackresources.EmptyStackState)
	assert.NoError(t, err)
	var (
		headless    *apiv1.Service
		published   *apiv1.Service
		randomPorts *apiv1.Service
	)
	for k, v := range s.Services {
		local := v
		if strings.HasSuffix(k, publishedServiceSuffix) {
			published = &local
		} else if strings.HasSuffix(k, publishedOnRandomPortSuffix) {
			randomPorts = &local
		} else {
			headless = &local
		}
	}
	return headless, published, randomPorts
}

func TestToServiceWithPublishedPort(t *testing.T) {
	headless, published, randomPorts := services(t, Stack("demo",
		WithService("nginx",
			Image("any"),
			WithPort(8080, Published(80)),
			WithLabel("container.key", "container.value"),
			Deploy(WithDeployLabel("deploy.key", "deploy.value")),
		),
	), loadBalancerServiceStrategy{})

	expectedHeadless := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx",
			Labels: map[string]string{
				"com.docker.stack.namespace": "demo",
				"com.docker.service.name":    "nginx",
				"com.docker.service.id":      "demo-nginx",
				"deploy.key":                 "deploy.value",
			},
		},
		Spec: apiv1.ServiceSpec{
			ClusterIP: apiv1.ClusterIPNone,
			Ports: []apiv1.ServicePort{
				{
					Name:       headlessPortName,
					Port:       headlessPort,
					Protocol:   apiv1.ProtocolTCP,
					TargetPort: intstr.FromInt(headlessPort),
				},
			},
			Selector: map[string]string{
				"com.docker.stack.namespace": "demo",
				"com.docker.service.name":    "nginx",
				"com.docker.service.id":      "demo-nginx",
			},
		},
	}
	assert.Equal(t, headless, expectedHeadless)

	expectedPublished := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("nginx%s", publishedServiceSuffix),
			Labels: map[string]string{
				"com.docker.stack.namespace": "demo",
				"com.docker.service.name":    "nginx",
				"com.docker.service.id":      "demo-nginx",
				"deploy.key":                 "deploy.value",
			},
		},
		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeLoadBalancer,
			Ports: []apiv1.ServicePort{
				{
					Name:       "80-tcp",
					Port:       80,
					Protocol:   apiv1.ProtocolTCP,
					TargetPort: intstr.FromInt(8080),
				},
			},
			Selector: map[string]string{
				"com.docker.stack.namespace": "demo",
				"com.docker.service.name":    "nginx",
				"com.docker.service.id":      "demo-nginx",
			},
		},
	}
	assert.Equal(t, published, expectedPublished)
	assert.Nil(t, randomPorts)
}

func TestToServiceWithLongPort(t *testing.T) {
	headless, published, randomPorts := services(t, Stack("demo",
		WithService("nginx",
			Image("any"),
			WithPort(8888, Published(80), ProtocolUDP),
		),
	), loadBalancerServiceStrategy{})

	assert.NotNil(t, headless)

	expectedPorts := []apiv1.ServicePort{
		{
			Name:       "80-udp",
			Port:       80,
			TargetPort: intstr.FromInt(8888),
			Protocol:   apiv1.ProtocolUDP,
		},
	}
	assert.Equal(t, published.Spec.Ports, expectedPorts)
	assert.Nil(t, randomPorts)
}

func TestToServiceWithNonPublishedPort(t *testing.T) {
	headless, published, randomPorts := services(t, Stack("demo",
		WithService("nginx",
			Image("any"),
		),
	), loadBalancerServiceStrategy{})

	expectedHeadless := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx",
			Labels: map[string]string{
				"com.docker.stack.namespace": "demo",
				"com.docker.service.name":    "nginx",
				"com.docker.service.id":      "demo-nginx",
			},
		},
		Spec: apiv1.ServiceSpec{
			ClusterIP: apiv1.ClusterIPNone,
			Ports: []apiv1.ServicePort{
				{
					Name:       headlessPortName,
					Port:       headlessPort,
					TargetPort: intstr.FromInt(headlessPort),
					Protocol:   apiv1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"com.docker.stack.namespace": "demo",
				"com.docker.service.name":    "nginx",
				"com.docker.service.id":      "demo-nginx",
			},
		},
	}
	assert.Equal(t, headless, expectedHeadless)

	assert.Nil(t, published)
	assert.Nil(t, randomPorts)
}

func TestToServiceWithRandomPublishedPort(t *testing.T) {
	headless, published, randomPorts := services(t, Stack("demo",
		WithService("nginx",
			Image("any"),
			WithPort(8888),
		),
	), loadBalancerServiceStrategy{})
	expectedHeadless := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx",
			Labels: map[string]string{
				"com.docker.stack.namespace": "demo",
				"com.docker.service.name":    "nginx",
				"com.docker.service.id":      "demo-nginx",
			},
		},
		Spec: apiv1.ServiceSpec{
			ClusterIP: apiv1.ClusterIPNone,
			Ports: []apiv1.ServicePort{
				{
					Name:       headlessPortName,
					Port:       headlessPort,
					TargetPort: intstr.FromInt(headlessPort),
					Protocol:   apiv1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"com.docker.stack.namespace": "demo",
				"com.docker.service.name":    "nginx",
				"com.docker.service.id":      "demo-nginx",
			},
		},
	}
	expectedRandomPorts := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("nginx%s", publishedOnRandomPortSuffix),
			Labels: map[string]string{
				"com.docker.stack.namespace": "demo",
				"com.docker.service.name":    "nginx",
				"com.docker.service.id":      "demo-nginx",
			},
		},
		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeNodePort,
			Ports: []apiv1.ServicePort{
				{
					Name:       "8888-tcp",
					Port:       8888,
					Protocol:   apiv1.ProtocolTCP,
					TargetPort: intstr.FromInt(8888),
				},
			},
			Selector: map[string]string{
				"com.docker.stack.namespace": "demo",
				"com.docker.service.name":    "nginx",
				"com.docker.service.id":      "demo-nginx",
			},
		},
	}
	assert.Equal(t, headless, expectedHeadless)
	assert.Nil(t, published)
	assert.Equal(t, randomPorts, expectedRandomPorts)
}

func TestToServiceWithoutStack(t *testing.T) {
	headless, published, randomPorts := services(t, Stack("demo",
		WithService("nginx",
			Image("any"),
			WithPort(8080, Published(8080)),
		),
	), loadBalancerServiceStrategy{})

	assert.NotNil(t, headless)

	expectedPublished := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("nginx%s", publishedServiceSuffix),
			Labels: map[string]string{
				"com.docker.stack.namespace": "demo",
				"com.docker.service.name":    "nginx",
				"com.docker.service.id":      "demo-nginx",
			},
		},
		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeLoadBalancer,
			Ports: []apiv1.ServicePort{
				{
					Name:       "8080-tcp",
					Port:       8080,
					Protocol:   apiv1.ProtocolTCP,
					TargetPort: intstr.FromInt(8080),
				},
			},
			Selector: map[string]string{
				"com.docker.stack.namespace": "demo",
				"com.docker.service.name":    "nginx",
				"com.docker.service.id":      "demo-nginx",
			},
		},
	}
	assert.Equal(t, published, expectedPublished)
	assert.Nil(t, randomPorts)
}

func TestToServiceWithPublishedPortWithNodePorts(t *testing.T) {
	headless, published, randomPorts := services(t, Stack("demo",
		WithService("nginx",
			Image("any"),
			WithPort(8080, Published(80)),
			WithLabel("container.key", "container.value"),
			Deploy(WithDeployLabel("deploy.key", "deploy.value")),
		),
	), nodePortServiceStrategy{})

	expectedHeadless := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx",
			Labels: map[string]string{
				"com.docker.stack.namespace": "demo",
				"com.docker.service.name":    "nginx",
				"com.docker.service.id":      "demo-nginx",
				"deploy.key":                 "deploy.value",
			},
		},
		Spec: apiv1.ServiceSpec{
			ClusterIP: apiv1.ClusterIPNone,
			Ports: []apiv1.ServicePort{
				{
					Name:       headlessPortName,
					Port:       headlessPort,
					Protocol:   apiv1.ProtocolTCP,
					TargetPort: intstr.FromInt(headlessPort),
				},
			},
			Selector: map[string]string{
				"com.docker.stack.namespace": "demo",
				"com.docker.service.name":    "nginx",
				"com.docker.service.id":      "demo-nginx",
			},
		},
	}
	assert.Equal(t, headless, expectedHeadless)

	expectedPublished := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("nginx%s", publishedServiceSuffix),
			Labels: map[string]string{
				"com.docker.stack.namespace": "demo",
				"com.docker.service.name":    "nginx",
				"com.docker.service.id":      "demo-nginx",
				"deploy.key":                 "deploy.value",
			},
		},
		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeNodePort,
			Ports: []apiv1.ServicePort{
				{
					Name:       "80-tcp",
					NodePort:   80,
					Port:       8080,
					Protocol:   apiv1.ProtocolTCP,
					TargetPort: intstr.FromInt(8080),
				},
			},
			Selector: map[string]string{
				"com.docker.stack.namespace": "demo",
				"com.docker.service.name":    "nginx",
				"com.docker.service.id":      "demo-nginx",
			},
		},
	}
	assert.Equal(t, published, expectedPublished)
	assert.Nil(t, randomPorts)
}

func TestToServiceWithLongPortWithNodePorts(t *testing.T) {
	headless, published, randomPorts := services(t, Stack("demo",
		WithService("nginx",
			Image("any"),
			WithPort(8888, Published(80), ProtocolUDP),
		),
	), nodePortServiceStrategy{})

	assert.NotNil(t, headless)

	expectedPorts := []apiv1.ServicePort{
		{
			Name:       "80-udp",
			NodePort:   80,
			Port:       8888,
			TargetPort: intstr.FromInt(8888),
			Protocol:   apiv1.ProtocolUDP,
		},
	}
	assert.Equal(t, published.Spec.Ports, expectedPorts)
	assert.Nil(t, randomPorts)
}

func TestToServiceWithNonPublishedPortWithNodePorts(t *testing.T) {
	headless, published, randomPorts := services(t, Stack("demo",
		WithService("nginx",
			Image("any"),
		),
	), nodePortServiceStrategy{})

	expectedHeadless := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx",
			Labels: map[string]string{
				"com.docker.stack.namespace": "demo",
				"com.docker.service.name":    "nginx",
				"com.docker.service.id":      "demo-nginx",
			},
		},
		Spec: apiv1.ServiceSpec{
			ClusterIP: apiv1.ClusterIPNone,
			Ports: []apiv1.ServicePort{
				{
					Name:       headlessPortName,
					Port:       headlessPort,
					TargetPort: intstr.FromInt(headlessPort),
					Protocol:   apiv1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"com.docker.stack.namespace": "demo",
				"com.docker.service.name":    "nginx",
				"com.docker.service.id":      "demo-nginx",
			},
		},
	}
	assert.Equal(t, headless, expectedHeadless)

	assert.Nil(t, published)
	assert.Nil(t, randomPorts)
}

func TestToServiceWithRandomPublishedPortWithNodePorts(t *testing.T) {
	headless, published, randomPorts := services(t, Stack("demo",
		WithService("nginx",
			Image("any"),
			WithPort(8888),
		),
	), nodePortServiceStrategy{})
	expectedHeadless := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx",
			Labels: map[string]string{
				"com.docker.stack.namespace": "demo",
				"com.docker.service.name":    "nginx",
				"com.docker.service.id":      "demo-nginx",
			},
		},
		Spec: apiv1.ServiceSpec{
			ClusterIP: apiv1.ClusterIPNone,
			Ports: []apiv1.ServicePort{
				{
					Name:       headlessPortName,
					Port:       headlessPort,
					TargetPort: intstr.FromInt(headlessPort),
					Protocol:   apiv1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"com.docker.stack.namespace": "demo",
				"com.docker.service.name":    "nginx",
				"com.docker.service.id":      "demo-nginx",
			},
		},
	}
	expectedRandomPorts := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("nginx%s", publishedOnRandomPortSuffix),
			Labels: map[string]string{
				"com.docker.stack.namespace": "demo",
				"com.docker.service.name":    "nginx",
				"com.docker.service.id":      "demo-nginx",
			},
		},
		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeNodePort,
			Ports: []apiv1.ServicePort{
				{
					Name:       "8888-tcp",
					Protocol:   apiv1.ProtocolTCP,
					Port:       8888,
					TargetPort: intstr.FromInt(8888),
				},
			},
			Selector: map[string]string{
				"com.docker.stack.namespace": "demo",
				"com.docker.service.name":    "nginx",
				"com.docker.service.id":      "demo-nginx",
			},
		},
	}
	assert.Equal(t, headless, expectedHeadless)
	assert.Nil(t, published)
	assert.Equal(t, randomPorts, expectedRandomPorts)
}

func TestToServiceWithoutStackWithNodePorts(t *testing.T) {
	headless, published, randomPorts := services(t, Stack("demo",
		WithService("nginx",
			Image("any"),
			WithPort(8080, Published(8080)),
		),
	), nodePortServiceStrategy{})

	assert.NotNil(t, headless)

	expectedPublished := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("nginx%s", publishedServiceSuffix),
			Labels: map[string]string{
				"com.docker.stack.namespace": "demo",
				"com.docker.service.name":    "nginx",
				"com.docker.service.id":      "demo-nginx",
			},
		},
		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeNodePort,
			Ports: []apiv1.ServicePort{
				{
					Name:       "8080-tcp",
					NodePort:   8080,
					Port:       8080,
					Protocol:   apiv1.ProtocolTCP,
					TargetPort: intstr.FromInt(8080),
				},
			},
			Selector: map[string]string{
				"com.docker.stack.namespace": "demo",
				"com.docker.service.name":    "nginx",
				"com.docker.service.id":      "demo-nginx",
			},
		},
	}
	assert.Equal(t, published, expectedPublished)
	assert.Nil(t, randomPorts)
}

func TestServiceStrategyFor(t *testing.T) {
	cases := []apiv1.ServiceType{
		apiv1.ServiceTypeLoadBalancer,
		apiv1.ServiceTypeNodePort,
	}
	for _, c := range cases {
		s, err := ServiceStrategyFor(c)
		assert.NoError(t, err)
		assert.Equal(t, c, s.publishedServiceType())
	}
}

func TestInternalServiceStrategy(t *testing.T) {
	cases := []struct {
		name              string
		desiredType       latest.InternalServiceType
		ports             []latest.InternalPort
		expectedClusterIP string
		original          apiv1.Service
	}{
		{
			name:              "auto-no-ports",
			expectedClusterIP: apiv1.ClusterIPNone,
		},
		{
			name: "auto-some-ports",
			ports: []latest.InternalPort{
				{
					Port:     8080,
					Protocol: apiv1.ProtocolUDP,
				},
			},
		},
		{
			name: "headless-some-ports",
			ports: []latest.InternalPort{
				{
					Port:     8080,
					Protocol: apiv1.ProtocolUDP,
				},
			},
			desiredType:       latest.InternalServiceTypeHeadless,
			expectedClusterIP: apiv1.ClusterIPNone,
		},
		{
			name:        "clusterip-no-ports",
			desiredType: latest.InternalServiceTypeClusterIP,
		},

		{
			name:              "auto-no-ports-existing-with-ip",
			expectedClusterIP: apiv1.ClusterIPNone,
			original: apiv1.Service{
				Spec: apiv1.ServiceSpec{
					ClusterIP: "1.1.1.1",
				},
			},
		},
		{
			name: "auto-some-ports-with-ip",
			ports: []latest.InternalPort{
				{
					Port:     8080,
					Protocol: apiv1.ProtocolUDP,
				},
			},
			original: apiv1.Service{
				Spec: apiv1.ServiceSpec{
					ClusterIP: "1.1.1.1",
				},
			},
			expectedClusterIP: "1.1.1.1",
		},
		{
			name: "auto-some-ports-no-ip",
			ports: []latest.InternalPort{
				{
					Port:     8080,
					Protocol: apiv1.ProtocolUDP,
				},
			},
			original: apiv1.Service{
				Spec: apiv1.ServiceSpec{
					ClusterIP: apiv1.ClusterIPNone,
				},
			},
			expectedClusterIP: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := toInternalService(metav1.ObjectMeta{}, nil, c.original, c.ports, c.desiredType)
			assert.Equal(t, c.expectedClusterIP, svc.Spec.ClusterIP)
		})
	}
}
