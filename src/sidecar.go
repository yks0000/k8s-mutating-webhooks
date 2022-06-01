package main

import corev1 "k8s.io/api/core/v1"

type NginxSideCarConfig struct {
	Name string
	ImageName string
	ImagePullPolicy corev1.PullPolicy
	Port int
	VolumeMounts []corev1.VolumeMount
}

func (config NginxSideCarConfig) SetContainerNameOrDefault() string {
	containerName := "nginx-webserver"
	if config.Name != "" {
		containerName = config.Name
	}
	return containerName
}

func generateNginxSideCarConfig(config NginxSideCarConfig, volumes []corev1.Volume) *Config {
	var containers []corev1.Container
	var nginxContainerPort []corev1.ContainerPort

	nginxContainer := corev1.Container{
		Name: config.SetContainerNameOrDefault(),
		Image: config.ImageName,
		ImagePullPolicy: config.ImagePullPolicy,
		Ports: append(nginxContainerPort, corev1.ContainerPort{
			ContainerPort: int32(config.Port),
			Protocol: "TCP",
		}),
		VolumeMounts: config.VolumeMounts,
	}

	sideCars := []corev1.Container{nginxContainer}
	containers = sideCars



	return &Config{
		Containers: containers,
		Volumes: volumes,
	}
}

func getPodVolumes() []corev1.Volume {
	var volumes []corev1.Volume

	volumes = append(volumes, corev1.Volume{
		Name: "nginx-tls",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: "sidecar-injector-certs",
			},
		},
	},
	)

	volumes = append(volumes, corev1.Volume{
		Name: "nginx-conf",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "nginx-conf",
				},
			},
		},
	},
	)

	return volumes
}

func getNginxSideCarConfig() *Config {
	var volumesMount []corev1.VolumeMount

	volumesMount = append(volumesMount, corev1.VolumeMount{
		Name: "nginx-conf",
		MountPath: "/etc/nginx/nginx.conf",
		SubPath: "nginx.conf",
	})
	volumesMount = append(volumesMount, corev1.VolumeMount{
		Name: "nginx-tls",
		MountPath: "/etc/nginx/ssl",
	})

	return generateNginxSideCarConfig(NginxSideCarConfig{
		ImagePullPolicy: corev1.PullAlways,
		ImageName: "nginx:stable",
		Port: 80,
		VolumeMounts: volumesMount,
	},
	getPodVolumes())
}