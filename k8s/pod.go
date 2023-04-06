package k8s

import (
	cachev1beta1 "bianchi2/dc-cache-backup-operator/api/v1beta1"
	"bianchi2/dc-cache-backup-operator/util"
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"time"
)

func GetRuntimePreWarmerPod(pod *corev1.Pod) *corev1.Pod {
	config, err := util.GetKubeConfig()
	if err != nil {
		return nil
	}
	// Create a Kubernetes client
	clientset, _ := kubernetes.NewForConfig(config)
	pod, err = clientset.CoreV1().Pods(pod.Namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
	if err != nil {
		return nil
	}
	return pod
}

// GetNewPreWarmerPod generates pre-warmer pod definition
func GetNewPreWarmerPod(cr *cachev1beta1.CacheBackupRequest, localHomePVCName string) *corev1.Pod {
	labels := cr.Spec.PodLabels
	labels["pvc"] = localHomePVCName
	if labels == nil {
		labels = make(map[string]string)
	}
	defaultMode := int32(0755)
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "prewarm-" + localHomePVCName,
			Namespace:   cr.Namespace,
			Labels:      labels,
			Annotations: cr.Spec.PodAnnotations,
		},
		Spec: corev1.PodSpec{
			RestartPolicy:             corev1.RestartPolicyNever,
			Tolerations:               cr.Spec.Tolerations,
			NodeSelector:              cr.Spec.NodeSelector,
			TopologySpreadConstraints: cr.Spec.TopologySpreadConstraints,
			Affinity:                  &cr.Spec.Affinity,
			Containers: []corev1.Container{
				{
					Name:    "pre-warmer",
					Image:   "atlassian/confluence:8.0.3",
					Command: []string{"/opt/script/copy-index.sh"},
					Env: []corev1.EnvVar{
						{
							Name:  "SHARED_HOME",
							Value: cr.Spec.SharedHomePath,
						},
						{
							Name:  "LOCAL_HOME",
							Value: cr.Spec.LocalHomePath,
						},
					},

					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "local-home",
							MountPath: cr.Spec.LocalHomePath,
						},
						{
							Name:      "shared-home",
							MountPath: cr.Spec.SharedHomePath,
						},
						{
							Name:      "copy-index",
							MountPath: "/opt/script",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "shared-home",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: cr.Spec.SharedHomePVCName,
						},
					},
				},
				{
					Name: "local-home",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: localHomePVCName,
						},
					},
				},
				{
					Name: "copy-index",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: cr.Spec.ConfigMapName,
							},
							DefaultMode: &defaultMode,
						},
					},
				},
			},
		},
	}
}

// WatchPodStatus watches a pod and returns its status
func WatchPodStatus(podName, namespace string) (string, error) {
	config, err := util.GetKubeConfig()
	if err != nil {
		return "", fmt.Errorf("error creating Kubernetes client: %v", err)
	}
	// Create a Kubernetes client
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", fmt.Errorf("error creating Kubernetes client: %v", err)
	}

	// Watch the pod for up to 10 minutes
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	watcher, err := clientset.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", podName),
	})
	if err != nil {
		return "", fmt.Errorf("error watching pod: %v", err)
	}

	// Wait for the pod status to become available
	for event := range watcher.ResultChan() {
		pod, ok := event.Object.(*corev1.Pod)
		if !ok {
			continue
		}
		if pod.Name == podName {
			switch pod.Status.Phase {
			case corev1.PodPending, corev1.PodRunning, corev1.PodSucceeded, corev1.PodFailed:
				return string(pod.Status.Phase), nil
			}
		}
	}

	return "", fmt.Errorf("timed out waiting for pod status")
}
