package controllers

import (
	cachev1beta1 "bianchi2/dc-cache-backup-operator/api/v1beta1"
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func GetNewPVC(cr *cachev1beta1.CacheBackupRequest, localHomePVCName string) *corev1.PersistentVolumeClaim {

	labels := cr.Spec.PVCLabels
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["app.kubernetes.io/instance"] = cr.Spec.InstanceName
	labels["app.kubernetes.io/name"] = cr.Spec.InstanceName

	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        localHomePVCName,
			Namespace:   cr.Namespace,
			Labels:      labels,
			Annotations: cr.Spec.PVCAnnotations,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(cr.Spec.PvcStorageRequest),
				},
			},
			StorageClassName: &cr.Spec.PvcStorageClass,
			Selector:         &cr.Spec.PvcLabelSelector,
			VolumeName:       cr.Spec.PvcVolumeName,
			VolumeMode:       (*corev1.PersistentVolumeMode)(&cr.Spec.PvcVolumeMode),
		},
	}
}

// IsPVCExistsAndFree returns PVC by name
func IsPVCExistsAndFree(cr *cachev1beta1.CacheBackupRequest, localHomePVCName string, clientset kubernetes.Interface) (exists bool, free bool, err error) {

	if err != nil {
		return false, false, err
	}

	// check if PVC exists
	pvc, err := clientset.CoreV1().PersistentVolumeClaims(cr.Namespace).Get(context.TODO(), localHomePVCName, metav1.GetOptions{})
	if err != nil || pvc == nil {
		return false, true, fmt.Errorf("PVC does not exist: %v", localHomePVCName)
	}

	// get all pods by label selector and check if PVC is used as volume source in volumes
	pods := &corev1.PodList{}
	pods, err = clientset.CoreV1().Pods(cr.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "app.kubernetes.io/name=" + cr.Spec.InstanceName})

	for _, pod := range pods.Items {
		volumes := pod.Spec.Volumes
		for _, volume := range volumes {
			if volume.VolumeSource.PersistentVolumeClaim != nil && volume.VolumeSource.PersistentVolumeClaim.ClaimName == localHomePVCName {
				return true, false, fmt.Errorf("PVC %v is used by pod %v", localHomePVCName, pod.Name)
			}
		}
	}
	return true, true, nil
}
