package convert

import (
	"fmt"

	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func toPersistentVolumeClaims(srv latest.ServiceConfig, original []apiv1.PersistentVolumeClaim) []apiv1.PersistentVolumeClaim {
	originalMap := make(map[string]apiv1.PersistentVolumeClaim)
	for _, c := range original {
		originalMap[c.Name] = c
	}
	var volumes []apiv1.PersistentVolumeClaim

	for i, volume := range srv.Volumes {
		if volume.Type != "volume" {
			continue
		}
		volumename := fmt.Sprintf("mount-%d", i)
		if volume.Source != "" {
			volumename = volume.Source
		}
		v := originalMap[volumename]
		v.ObjectMeta = metav1.ObjectMeta{
			Name:        volumename,
			Annotations: srv.Deploy.Labels,
		}
		v.Spec.AccessModes = []apiv1.PersistentVolumeAccessMode{
			apiv1.ReadWriteOnce,
		}
		v.Spec.Resources = apiv1.ResourceRequirements{
			Requests: apiv1.ResourceList{
				apiv1.ResourceStorage: resource.MustParse("100Mi"),
			},
		}
		volumes = append(volumes, v)
	}

	return volumes
}
