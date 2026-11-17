package configmap

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDesired(objectMeta metav1.ObjectMeta, someDummyData map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: objectMeta,
		Data:       someDummyData,
	}
}
