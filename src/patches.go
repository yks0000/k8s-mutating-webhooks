package main

import corev1 "k8s.io/api/core/v1"

func addContainer(target, containers []corev1.Container, basePath string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}

	for _, add := range containers {
		value = add
		path := basePath
		if first {
			first = false
			value = []corev1.Container{add}
		} else {
			path = path + "/-"
		}
		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}

	return patch
}

func addVolume(target, volumes []corev1.Volume, basePath string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}

	for _, add := range volumes {
		value = add
		path := basePath

		if first {
			first = false
			value = []corev1.Volume{add}
		} else {
			path = path + "/-"
		}

		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}

	return patch
}


func createPatch(pod corev1.Pod, sidecarConfig *Config) ([]patchOperation, error) {
	var patches []patchOperation
	patches = append(patches, addContainer(pod.Spec.Containers, sidecarConfig.Containers, "/spec/containers")...)
	patches = append(patches, addVolume(pod.Spec.Volumes, sidecarConfig.Volumes, "/spec/volumes")...)

	labels := pod.ObjectMeta.Labels
	labels["nginx-sidecar"] = "applied-from-mutating-webhook"

	patches = append(patches, patchOperation{
		Op:    "add",
		Path:  "/metadata/labels",
		Value: labels,
	})

	return patches, nil

}