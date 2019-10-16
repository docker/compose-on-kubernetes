package install

import (
	"context"
	"fmt"

	corev1types "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

// Safe installs the compose features securely
func Safe(ctx context.Context, config *rest.Config, options SafeOptions) error {
	return Do(ctx, config, WithSafe(options))
}

func (c *installer) createEtcdSecret(*installerContext) error {
	if c.etcdOptions == nil {
		return nil
	}
	update := true
	secret, err := c.coreClient.Secrets(c.commonOptions.Namespace).Get("compose-etcd", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		update = false
	} else if err != nil {
		return err
	}
	if secret == nil {
		secret = &corev1types.Secret{}
	}
	secret.Name = "compose-etcd"
	secret.Labels = c.apiLabels
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	secret.Data["servers"] = []byte(c.etcdOptions.Servers)
	if c.etcdOptions.ClientTLSBundle != nil {
		secret.Data["ca"] = c.etcdOptions.ClientTLSBundle.ca
		secret.Data["cert"] = c.etcdOptions.ClientTLSBundle.cert
		secret.Data["key"] = c.etcdOptions.ClientTLSBundle.key
	}
	shouldDo, err := c.objectFilter.filter(secret)
	if err != nil {
		return err
	}
	if shouldDo {
		if update {
			_, err := c.coreClient.Secrets(c.commonOptions.Namespace).Update(secret)
			return err
		}
		_, err = c.coreClient.Secrets(c.commonOptions.Namespace).Create(secret)
		return err
	}
	return nil
}

func (c *installer) createNetworkSecret(_ *installerContext) error {
	if c.networkOptions == nil || c.networkOptions.CustomTLSBundle == nil {
		return nil
	}
	update := true
	secret, err := c.coreClient.Secrets(c.commonOptions.Namespace).Get("compose-tls", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		update = false
	} else if err != nil {
		return err
	}
	if secret == nil {
		secret = &corev1types.Secret{}
	}
	secret.Name = "compose-tls"
	secret.Labels = c.apiLabels
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	secret.Data["ca"] = c.networkOptions.CustomTLSBundle.ca
	secret.Data["cert"] = c.networkOptions.CustomTLSBundle.cert
	secret.Data["key"] = c.networkOptions.CustomTLSBundle.key

	shouldDo, err := c.objectFilter.filter(secret)
	if err != nil {
		return err
	}
	if shouldDo {
		if update {
			_, err = c.coreClient.Secrets(c.commonOptions.Namespace).Update(secret)
			return err
		}
		_, err = c.coreClient.Secrets(c.commonOptions.Namespace).Create(secret)
		return err
	}
	return nil
}

func applyEtcdOptions(pod *corev1types.PodSpec, opts *EtcdOptions) {
	if opts == nil {
		// unsafe case
		pod.Containers[0].Args = append(pod.Containers[0].Args, "--etcd-servers=http://127.0.0.1:2379")
		pod.Containers = append(pod.Containers, corev1types.Container{
			Name:            "etcd",
			Image:           "quay.io/coreos/etcd:v3.3.15",
			ImagePullPolicy: corev1types.PullAlways,
			Args: []string{
				"/usr/local/bin/etcd",
				"-advertise-client-urls=http://127.0.0.1:2379",
				"-listen-client-urls=http://127.0.0.1:2379",
			},
		})
		return
	}
	pod.Containers[0].Args = append(pod.Containers[0].Args, "--etcd-servers="+opts.Servers)
	if opts.ClientTLSBundle == nil {
		return
	}
	svs := &corev1types.SecretVolumeSource{
		SecretName: "compose-etcd",
	}
	if opts.ClientTLSBundle.ca != nil {
		svs.Items = append(svs.Items, corev1types.KeyToPath{
			Key:  "ca",
			Path: "ca.crt",
		})
		pod.Containers[0].Args = append(pod.Containers[0].Args, "--etcd-cafile=/etc/docker-compose/etcd/ca.crt")
	}
	if opts.ClientTLSBundle.cert != nil {
		svs.Items = append(svs.Items, corev1types.KeyToPath{
			Key:  "cert",
			Path: "client.crt",
		})
		pod.Containers[0].Args = append(pod.Containers[0].Args, "--etcd-certfile=/etc/docker-compose/etcd/client.crt")
	}
	if opts.ClientTLSBundle.key != nil {
		svs.Items = append(svs.Items, corev1types.KeyToPath{
			Key:  "key",
			Path: "client.key",
		})
		pod.Containers[0].Args = append(pod.Containers[0].Args, "--etcd-keyfile=/etc/docker-compose/etcd/client.key")
	}
	pod.Volumes = append(pod.Volumes, corev1types.Volume{
		Name:         "etcd-secret",
		VolumeSource: corev1types.VolumeSource{Secret: svs},
	})
	pod.Containers[0].VolumeMounts = append(pod.Containers[0].VolumeMounts, corev1types.VolumeMount{
		Name:      "etcd-secret",
		MountPath: "/etc/docker-compose/etcd",
		ReadOnly:  true,
	})
}

func applyNetworkOptions(pod *corev1types.PodSpec, opts *NetworkOptions) {
	if opts == nil {
		pod.Containers[0].Ports = append(pod.Containers[0].Ports, corev1types.ContainerPort{
			Name:          "api",
			ContainerPort: 9443,
		})
		pod.Containers[0].Args = append(pod.Containers[0].Args, "--secure-port", "9443")
		return
	}

	if opts.Port == 0 {
		opts.Port = 9443
	}
	if opts.ShouldUseHost {
		pod.HostNetwork = true
	} else {
		pod.Containers[0].Ports = append(pod.Containers[0].Ports, corev1types.ContainerPort{
			Name:          "api",
			ContainerPort: opts.Port,
		})
	}
	pod.Containers[0].Args = append(pod.Containers[0].Args, fmt.Sprintf("--secure-port=%v", opts.Port))

	if opts.CustomTLSBundle == nil {
		return
	}
	svs := &corev1types.SecretVolumeSource{
		SecretName: "compose-tls",
		Items: []corev1types.KeyToPath{
			{
				Key:  "ca",
				Path: "ca.crt",
			},
			{
				Key:  "cert",
				Path: "server.crt",
			},
			{
				Key:  "key",
				Path: "server.key",
			},
		},
	}
	pod.Volumes = append(pod.Volumes, corev1types.Volume{
		Name:         "tls-secret",
		VolumeSource: corev1types.VolumeSource{Secret: svs},
	})
	pod.Containers[0].VolumeMounts = append(pod.Containers[0].VolumeMounts, corev1types.VolumeMount{
		Name:      "tls-secret",
		MountPath: "/etc/docker-compose/tls",
		ReadOnly:  true,
	})
	pod.Containers[0].Args = append(pod.Containers[0].Args,
		"--tls-cert-file=/etc/docker-compose/tls/server.crt",
		"--tls-private-key-file=/etc/docker-compose/tls/server.key",
		"--ca-bundle-file=/etc/docker-compose/tls/ca.crt",
	)

}
