/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CacheBackupRequestSpec defines the desired state of CacheBackupRequest
type CacheBackupRequestSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	SharedHomePVCName     string `json:"sharedHomePVCName,omitempty"`
	IndexSnapshotsPath    string `json:"indexSnapshotsPath,omitempty"`
	SharedHomePath        string `json:"sharedHomePath,omitempty"`
	LocalHomePath         string `json:"localHomePath,omitempty"`
	InstanceName          string `json:"instanceName,omitempty"`
	StatefulSetNumber     int    `json:"statefulSetNumber,omitempty"`
	BackupIntervalMinutes int    `json:"backupIntervalMinutes,omitempty"`
	ConfigMapName         string `json:"configMapName,omitempty"`

	PodLabels                 map[string]string                 `json:"podLabels,omitempty"`
	PodAnnotations            map[string]string                 `json:"podAnnotations,omitempty"`
	PodResources              corev1.ResourceRequirements       `json:"podResources,omitempty"`
	NodeSelector              map[string]string                 `json:"nodeSelector,omitempty"`
	Tolerations               []corev1.Toleration               `json:"tolerations,omitempty"`
	Affinity                  corev1.Affinity                   `json:"affinity,omitempty"`
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`

	CreatePVC         bool                 `json:"createPVC,omitempty"`
	PVCLabels         map[string]string    `json:"pvcLabels,omitempty"`
	PVCAnnotations    map[string]string    `json:"pvcAnnotations,omitempty"`
	PvcStorageRequest string               `json:"pvcStorageRequest,omitempty"`
	PvcStorageClass   string               `json:"pvcStorageClass,omitempty"`
	PvcVolumeName     string               `json:"pvcVolumeName,omitempty"`
	PvcVolumeMode     string               `json:"pvcVolumeMode,omitempty"`
	PvcLabelSelector  metav1.LabelSelector `json:"pvcLabelSelector,omitempty"`
}

// CacheBackupRequestStatus defines the observed state of CacheBackupRequest
type CacheBackupRequestStatus struct {

	// Name of the PVC
	PVCName string `json:"pvcName,omitempty"`

	// Status of the PVC
	Status string `json:"status,omitempty"`

	// Timestamp for last transaction
	LastTransactionTime string `json:"lastTransactionTime,omitempty"`

	IndexRestoreDurationSeconds int `json:"indexRestoreDurationSeconds,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CacheBackupRequest is the Schema for the cachebackuprequests API
type CacheBackupRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CacheBackupRequestSpec   `json:"spec,omitempty"`
	Status CacheBackupRequestStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CacheBackupRequestList contains a list of CacheBackupRequest
type CacheBackupRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CacheBackupRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CacheBackupRequest{}, &CacheBackupRequestList{})
}
