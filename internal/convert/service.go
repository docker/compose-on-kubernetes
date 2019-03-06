package convert

import (
	"fmt"
	"strings"

	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	"github.com/docker/compose-on-kubernetes/internal/stackresources"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// publishedServiceSuffix is the suffix given to services that are exposed
	// externally.
	publishedServiceSuffix      = "-published"
	publishedOnRandomPortSuffix = "-random-ports"
	// headlessPortName is the name given to the port for headless services.
	headlessPortName = "headless"
	// headlessPort is the port allocated for headless services.
	headlessPort = 55555
)

// ServiceStrategy defines a strategy to use for converting stacks ports to kubernetes services
type ServiceStrategy interface {
	randomServiceType() apiv1.ServiceType
	publishedServiceType() apiv1.ServiceType
	convertServicePort(config latest.ServicePortConfig, originalPublished, originalRandom []apiv1.ServicePort) (port apiv1.ServicePort, published bool)
}

// ServiceStrategyFor returns the correct strategy for a desired publishServiceType
func ServiceStrategyFor(publishServiceType apiv1.ServiceType) (ServiceStrategy, error) {
	switch publishServiceType {
	case apiv1.ServiceTypeLoadBalancer:
		return loadBalancerServiceStrategy{}, nil
	case apiv1.ServiceTypeNodePort:
		return nodePortServiceStrategy{}, nil
	}
	return nil, fmt.Errorf("No strategy for service type %s", publishServiceType)
}

func findPortOrDefault(ports []apiv1.ServicePort, name string) apiv1.ServicePort {
	for _, p := range ports {
		if p.Name == name {
			return p
		}
	}
	return apiv1.ServicePort{
		Name: name,
	}
}

type loadBalancerServiceStrategy struct{}

func (loadBalancerServiceStrategy) randomServiceType() apiv1.ServiceType {
	return apiv1.ServiceTypeNodePort
}

func (loadBalancerServiceStrategy) publishedServiceType() apiv1.ServiceType {
	return apiv1.ServiceTypeLoadBalancer
}

func (loadBalancerServiceStrategy) convertServicePort(source latest.ServicePortConfig, originalPublished, originalRandom []apiv1.ServicePort) (port apiv1.ServicePort, published bool) {
	proto := toProtocol(source.Protocol)
	protoLower := strings.ToLower(string(proto))
	if source.Published != 0 {
		p := findPortOrDefault(originalPublished, fmt.Sprintf("%d-%s", source.Published, protoLower))
		p.Port = int32(source.Published)
		p.Protocol = proto
		p.TargetPort = intstr.FromInt(int(source.Target))
		return p, true
	}

	p := findPortOrDefault(originalRandom, fmt.Sprintf("%d-%s", source.Target, protoLower))
	p.Protocol = proto
	p.Port = int32(source.Target)
	p.TargetPort = intstr.FromInt(int(source.Target))
	return p, false

}

type nodePortServiceStrategy struct{}

func (nodePortServiceStrategy) randomServiceType() apiv1.ServiceType {
	return apiv1.ServiceTypeNodePort
}

func (nodePortServiceStrategy) publishedServiceType() apiv1.ServiceType {
	return apiv1.ServiceTypeNodePort
}

func (nodePortServiceStrategy) convertServicePort(source latest.ServicePortConfig, originalPublished, originalRandom []apiv1.ServicePort) (port apiv1.ServicePort, published bool) {
	proto := toProtocol(source.Protocol)
	protoLower := strings.ToLower(string(proto))
	if source.Published != 0 {
		p := findPortOrDefault(originalPublished, fmt.Sprintf("%d-%s", source.Published, protoLower))
		p.NodePort = int32(source.Published)
		p.Protocol = proto
		p.Port = int32(source.Target)
		p.TargetPort = intstr.FromInt(int(source.Target))
		return p, true
	}
	p := findPortOrDefault(originalRandom, fmt.Sprintf("%d-%s", source.Target, protoLower))
	p.Protocol = proto
	p.TargetPort = intstr.FromInt(int(source.Target))
	p.Port = int32(source.Target)
	return p, false
}

// toServices converts a Compose Service to a Kubernetes headless service as
// well as a normal service if it requires published ports.
func toServices(s latest.ServiceConfig, objectMeta metav1.ObjectMeta, labelSelector map[string]string,
	strategy ServiceStrategy, original *stackresources.StackState) (*apiv1.Service, *apiv1.Service, *apiv1.Service) {
	headlessMeta := objectMeta
	publishedMeta := publishedObjectMeta(objectMeta)
	randomPortsMeta := randomPortsObjectMeta(objectMeta)

	originalHL := original.Services[stackresources.ObjKey(headlessMeta.Namespace, headlessMeta.Name)]
	hl := toInternalService(headlessMeta, labelSelector, originalHL, s.InternalPorts, s.InternalServiceType)

	originalRandom := original.Services[stackresources.ObjKey(randomPortsMeta.Namespace, randomPortsMeta.Name)]
	originalPublished := original.Services[stackresources.ObjKey(publishedMeta.Namespace, publishedMeta.Name)]

	var randomPorts, publishedPorts []apiv1.ServicePort
	for _, p := range s.Ports {
		port, published := strategy.convertServicePort(p, originalPublished.Spec.Ports, originalRandom.Spec.Ports)
		if published {
			publishedPorts = append(publishedPorts, port)
		} else {
			randomPorts = append(randomPorts, port)
		}
	}
	published := toExposedService(
		publishedMeta,
		publishedPorts,
		strategy.publishedServiceType(),
		labelSelector,
		originalPublished,
	)
	random := toExposedService(
		randomPortsMeta,
		randomPorts,
		strategy.randomServiceType(),
		labelSelector,
		originalRandom,
	)
	return hl, published, random
}

// toInternalService creates a Kubernetes service for intra-stack communication.
func toInternalService(objectMeta metav1.ObjectMeta, labelSelector map[string]string, original apiv1.Service,
	internalPorts []latest.InternalPort, internalServiceType latest.InternalServiceType) *apiv1.Service {
	useHeadless := false
	switch internalServiceType {
	case latest.InternalServiceTypeHeadless:
		useHeadless = true
	case latest.InternalServiceTypeAuto:
		useHeadless = len(internalPorts) == 0
	}
	service := original.DeepCopy()
	service.ObjectMeta = objectMeta
	service.Spec.Selector = labelSelector
	if useHeadless {
		service.Spec.ClusterIP = apiv1.ClusterIPNone
		service.Spec.Ports = []apiv1.ServicePort{{
			Name:       headlessPortName,
			Port:       headlessPort,
			TargetPort: intstr.FromInt(headlessPort),
			Protocol:   apiv1.ProtocolTCP,
		}}
	} else {
		if service.Spec.ClusterIP == apiv1.ClusterIPNone {
			service.Spec.ClusterIP = ""
		}
		service.Spec.Ports = []apiv1.ServicePort{}
		for _, p := range internalPorts {
			service.Spec.Ports = append(service.Spec.Ports,
				apiv1.ServicePort{
					Name:       fmt.Sprintf("%d-%s", p.Port, strings.ToLower(string(p.Protocol))),
					Port:       p.Port,
					TargetPort: intstr.FromInt(int(p.Port)),
					Protocol:   p.Protocol,
				})
		}
	}
	return service
}

// toExposedService creates a Kubernetes service with exposed ports. The
// service name is suffixed to distinguish it from a headless service.
func toExposedService(objectMeta metav1.ObjectMeta, servicePorts []apiv1.ServicePort, svcType apiv1.ServiceType, labelSelector map[string]string, original apiv1.Service) *apiv1.Service {
	if len(servicePorts) == 0 {
		return nil
	}
	service := original.DeepCopy()
	service.ObjectMeta = objectMeta
	service.Spec.Type = svcType
	service.Spec.Selector = labelSelector
	service.Spec.Ports = servicePorts
	return service
}

// publishedObjectMeta appends "-published" to the name of a given Kubernetes
// object metadata.
func publishedObjectMeta(objectMeta metav1.ObjectMeta) metav1.ObjectMeta {
	res := objectMeta
	res.Name = fmt.Sprintf("%s%s", objectMeta.Name, publishedServiceSuffix)
	return res
}

func randomPortsObjectMeta(objectMeta metav1.ObjectMeta) metav1.ObjectMeta {
	res := objectMeta
	res.Name = fmt.Sprintf("%s%s", objectMeta.Name, publishedOnRandomPortSuffix)
	return res
}
