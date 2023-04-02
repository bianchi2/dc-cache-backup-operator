package k8s

import (
	cachev1beta1 "bianchi2/dc-cache-backup-operator/api/v1beta1"
	"bianchi2/dc-cache-backup-operator/util"
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"time"
)

// GetNewPreWarmerPod generates pre-warmer pod definition
func GetNewPreWarmerPod(cr *cachev1beta1.CacheBackupRequest, localHomePVCName string) *corev1.Pod {
	labels := map[string]string{
		"pvc": localHomePVCName,
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      util.GenerateK8sCompliantName("prewarm-"+localHomePVCName, 7),
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:    "pre-warmer",
					Image:   "atlassian/confluence:8.0.3",
					Command: []string{"/bin/bash"},
					Args:    []string{"-c", "sleep 5 && ls -la ${CONFLUENCE_HOME} && ls -la /var/atlassian/application-data/shared-home/index-snapshots"},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "local-home",
							MountPath: "/var/atlassian/application-data/confluence",
						},
						{
							Name:      "shared-home",
							MountPath: cr.Spec.IndexSnapshotsPath,
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
			},
		},
	}
}

// WatchPodStatus watches a pod and returns its status
func WatchPodStatus(podName, namespace string) (string, error) {
	// Load Kubernetes configuration from the default location
	fmt.Println("Starting watching pod " + podName)

	kubeconfig := os.Getenv("KUBECONFIG")
	//config, err := rest.InClusterConfig()
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)

	if err != nil {
		return "", fmt.Errorf("error loading Kubernetes config: %v", err)
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
			case corev1.PodSucceeded, corev1.PodFailed:
				return string(pod.Status.Phase), nil
			}
		}
	}

	return "", fmt.Errorf("timed out waiting for pod status")
}
