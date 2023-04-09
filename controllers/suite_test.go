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

package controllers

import (
	"bianchi2/dc-cache-backup-operator/k8s"
	"context"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	testclient "k8s.io/client-go/kubernetes/fake"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"strconv"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	cachev1beta1 "bianchi2/dc-cache-backup-operator/api/v1beta1"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment

//func TestAPIs(t *testing.T) {
//	RegisterFailHandler(Fail)
//
//	RunSpecs(t, "Controller Suite")
//}

const (
	layout                 = "2006-01-02 15:04:05 -0700"
	testCustomResourceName = "sample-backup-request"
	instanceName           = "confluence"
	namespace              = "default"
	statefulSetNumberOne   = 1
	statefulSetNumberTwo   = 2
)

var metadataMap = map[string]string{"foo": "bar"}
var podAffinityTerm = &corev1.PodAffinityTerm{
	LabelSelector: &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      "app",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{"nginx"},
			},
		},
	},
	TopologyKey: "kubernetes.io/hostname",
}
var fakeClient = fake.NewClientBuilder().Build()
var testClient = testclient.NewSimpleClientset()

var instanceCreatePVC = cachev1beta1.CacheBackupRequest{
	ObjectMeta: metav1.ObjectMeta{
		Name:      testCustomResourceName,
		Namespace: namespace,
	},
	Spec: cachev1beta1.CacheBackupRequestSpec{
		InstanceName:          instanceName,
		StatefulSetNumber:     statefulSetNumberOne,
		CreatePVC:             true,
		PvcStorageRequest:     "1Gi",
		BackupIntervalMinutes: 1,
		PVCLabels:             metadataMap,
		PVCAnnotations:        metadataMap,
		PvcVolumeName:         instanceName,
		PvcStorageClass:       instanceName,
		PvcLabelSelector: metav1.LabelSelector{
			MatchLabels: metadataMap,
		},
		PvcVolumeMode:  instanceName,
		PodLabels:      metadataMap,
		PodAnnotations: metadataMap,
		NodeSelector:   metadataMap,
		Affinity: corev1.Affinity{
			NodeAffinity: nil,
			PodAffinity: &corev1.PodAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{*podAffinityTerm},
			},
			PodAntiAffinity: nil,
		},
		Tolerations: []corev1.Toleration{
			{
				Key:               "example-key",
				Operator:          "Equal",
				Value:             "example-value",
				Effect:            "NoSchedule",
				TolerationSeconds: nil,
			},
		},
		TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
			{
				MaxSkew:            1,
				TopologyKey:        "kubernetes.io/hostname",
				WhenUnsatisfiable:  "DoNotSchedule",
				LabelSelector:      &metav1.LabelSelector{MatchLabels: map[string]string{"example-key": "example-value"}},
				MinDomains:         nil,
				NodeAffinityPolicy: nil,
				NodeTaintsPolicy:   nil,
				MatchLabelKeys:     []string{"example-key"},
			},
		},
	},
}

var instanceUseExistingPVC = cachev1beta1.CacheBackupRequest{
	ObjectMeta: metav1.ObjectMeta{
		Name:      testCustomResourceName + "-" + strconv.Itoa(statefulSetNumberTwo),
		Namespace: namespace,
	},
	Spec: cachev1beta1.CacheBackupRequestSpec{
		InstanceName:      instanceName,
		StatefulSetNumber: statefulSetNumberTwo,
		CreatePVC:         false,
		PvcStorageRequest: "1Gi",
	},
}

var cacheBackupRequestReconcilerPodRunning = CacheBackupRequestReconciler{
	Client:    fakeClient,
	Scheme:    scheme.Scheme,
	K8sClient: testClient,
	Test: TestSuite{
		Test:  true,
		State: string(corev1.PodRunning),
	},
}

var cacheBackupRequestReconcilerPodSucceeded = CacheBackupRequestReconciler{
	Client:    fakeClient,
	Scheme:    scheme.Scheme,
	K8sClient: testClient,
	Test: TestSuite{
		Test:  true,
		State: string(corev1.PodSucceeded),
	},
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = cachev1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func init() {
	// we need to add custom resource to known types for the fake client
	s := scheme.Scheme
	s.AddKnownTypes(cachev1beta1.GroupVersion, &cachev1beta1.CacheBackupRequest{})
}

func TestRunningSucceededPod(t *testing.T) {
	sampleBackupRequest := &instanceCreatePVC
	err := fakeClient.Create(context.TODO(), sampleBackupRequest)
	r := &cacheBackupRequestReconcilerPodRunning
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      testCustomResourceName,
			Namespace: namespace,
		},
	}
	ctx := context.Background()
	res, err := r.Reconcile(ctx, req)

	assert.NoError(t, err)
	assert.Equal(t, reconcile.Result{RequeueAfter: 1 * time.Second}, res)

	// Check that the PVC was created.
	createdPVC := &corev1.PersistentVolumeClaim{}
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "local-home-" + instanceName + "-" + strconv.Itoa(statefulSetNumberOne), Namespace: namespace}, createdPVC)
	assert.NoError(t, err)

	// Check that the pod was created.
	createdPod := &corev1.Pod{}
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "prewarm-local-home-" + instanceName + "-" + strconv.Itoa(statefulSetNumberOne), Namespace: namespace}, createdPod)
	assert.NoError(t, err)

	// assert pod labels
	podLabels := createdPod.Labels
	value, exists := podLabels["foo"]
	assert.True(t, exists)
	assert.Equal(t, "bar", value)

	// assert pod annotations
	podAnnotations := createdPod.Annotations
	value, exists = podAnnotations["foo"]
	assert.True(t, exists)
	assert.Equal(t, "bar", value)

	// assert node selector
	nodeSelector := createdPod.Spec.NodeSelector
	value, exists = nodeSelector["foo"]
	assert.True(t, exists)
	assert.Equal(t, "bar", value)

	// assert pod affinity is not nil
	podAffinity := createdPod.Spec.Affinity.PodAffinity
	assert.NotNil(t, podAffinity)

	//assert tolerations
	tolerationsKey := createdPod.Spec.Tolerations[0].Key
	tolerationsValue := createdPod.Spec.Tolerations[0].Value
	assert.Equal(t, "example-key", tolerationsKey)
	assert.Equal(t, "example-value", tolerationsValue)

	// assert TopologySpreadConstraints
	topologySpreadConstraintKey := createdPod.Spec.TopologySpreadConstraints[0].TopologyKey
	topologySpreadConstraintMaxSkew := createdPod.Spec.TopologySpreadConstraints[0].MaxSkew
	topologySpreadConstraintWhenUnsatisfiable := createdPod.Spec.TopologySpreadConstraints[0].WhenUnsatisfiable

	assert.Equal(t, "kubernetes.io/hostname", topologySpreadConstraintKey)
	assert.Equal(t, int32(1), topologySpreadConstraintMaxSkew)
	assert.Equal(t, "DoNotSchedule", string(topologySpreadConstraintWhenUnsatisfiable))

	// check that custom resource status is Running
	instance := &cachev1beta1.CacheBackupRequest{}
	err = fakeClient.Get(ctx, types.NamespacedName{Name: testCustomResourceName, Namespace: namespace}, instance)
	assert.Equal(t, string(corev1.PodRunning), instance.Status.Status)

	// check if PVC has got expected labels
	pvcLabels := createdPVC.Labels
	value, exists = pvcLabels["foo"]
	assert.True(t, exists)
	assert.Equal(t, "bar", value)

	// check if PVC has got expected annotations
	pvcAnnotations := createdPVC.Annotations
	value, exists = pvcAnnotations["foo"]
	assert.True(t, exists)
	assert.Equal(t, "bar", value)

	// check if PVC has expected storage class
	storageClassName := *createdPVC.Spec.StorageClassName
	assert.Equal(t, instanceName, storageClassName)

	// check if PVC has the expected volume mode
	volumeMode := createdPVC.Spec.VolumeMode
	assert.Equal(t, instanceName, string(*volumeMode))

	// check if PVC has the expected volume name
	volumeName := createdPVC.Spec.VolumeName
	assert.Equal(t, instanceName, volumeName)

	// pass a fake Succeeded state and reconcile
	r = &cacheBackupRequestReconcilerPodSucceeded
	req = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      testCustomResourceName,
			Namespace: namespace,
		},
	}
	res, err = r.Reconcile(ctx, req)

	assert.NoError(t, err)
	assert.Equal(t, reconcile.Result{RequeueAfter: 30 * time.Second}, res)

	// assert custom resource status has been updated to Succeeded
	err = fakeClient.Get(ctx, types.NamespacedName{Name: testCustomResourceName, Namespace: namespace}, instance)
	assert.Equal(t, string(corev1.PodSucceeded), instance.Status.Status)

	// assert that IndexRestoreDurationSeconds is greater than 0
	assert.Greater(t, instance.Status.IndexRestoreDurationSeconds, 0)

	// assert the pod has been deleted
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "prewarm-local-home-" + instanceName + "-" + strconv.Itoa(statefulSetNumberOne), Namespace: namespace}, createdPod)
	assert.Error(t, err)

	// reconcile immediately to verify that the controller does not create a pod
	// because BackupIntervalMinutes is set to 1 minute
	res, err = r.Reconcile(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, reconcile.Result{RequeueAfter: 65 * time.Second}, res)

	// update custom resource status.LastTransactionTime to initiate creation of a new pre-warmer pod
	// BackupIntervalMinutes is set to 1 minute, so we update the status LastTransactionTime to be 61 seconds behind
	lastTransactionTime := instance.Status.LastTransactionTime
	parsedTime, err := time.Parse(layout, lastTransactionTime)
	assert.NoError(t, err)

	timeInPast := parsedTime.Add(-61 * time.Second)
	instance.Status.LastTransactionTime = timeInPast.Format(layout)
	err = r.Client.Status().Update(ctx, instance)
	assert.NoError(t, err)

	// set pod status to running and reconcile
	r = &cacheBackupRequestReconcilerPodRunning
	res, err = r.Reconcile(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, reconcile.Result{RequeueAfter: 1 * time.Second}, res)

	// Check that the pod was created.
	createdPod = &corev1.Pod{}
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "prewarm-local-home-" + instanceName + "-" + strconv.Itoa(statefulSetNumberOne), Namespace: namespace}, createdPod)
	assert.NoError(t, err)

	// check that custom resource status is Running
	instance = &cachev1beta1.CacheBackupRequest{}
	err = fakeClient.Get(ctx, types.NamespacedName{Name: testCustomResourceName, Namespace: namespace}, instance)
	assert.Equal(t, string(corev1.PodRunning), instance.Status.Status)
}

func TestPVCDoesNotExist(t *testing.T) {
	requestNoPVC := &instanceUseExistingPVC
	err := fakeClient.Create(context.TODO(), requestNoPVC)
	r := &cacheBackupRequestReconcilerPodRunning
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      testCustomResourceName + "-" + strconv.Itoa(statefulSetNumberTwo),
			Namespace: namespace,
		},
	}

	ctx := context.Background()
	res, err := r.Reconcile(ctx, req)

	// Check that the PVC does not exist and the error is thrown
	createdPVC := &corev1.PersistentVolumeClaim{}
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "local-home-" + instanceName + "-" + strconv.Itoa(statefulSetNumberTwo), Namespace: namespace}, createdPVC)
	assert.Error(t, err)

	assert.Error(t, err)
	assert.Equal(t, reconcile.Result{RequeueAfter: 1 * time.Minute}, res)

	// Check that the pod was not created.
	createdPod := &corev1.Pod{}
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "prewarm-local-home-" + instanceName + "-" + strconv.Itoa(statefulSetNumberTwo), Namespace: namespace}, createdPod)
	assert.Error(t, err)

	instance := &cachev1beta1.CacheBackupRequest{}
	err = fakeClient.Get(ctx, types.NamespacedName{Name: testCustomResourceName + "-" + strconv.Itoa(statefulSetNumberTwo), Namespace: namespace}, instance)

	assert.Equal(t, "PVCDoesNotExist", instance.Status.Status)
}

func TestPVCBeingCurrentlyUsed(t *testing.T) {
	sampleBackupRequest := &instanceUseExistingPVC
	err := fakeClient.Create(context.TODO(), sampleBackupRequest)
	r := &cacheBackupRequestReconcilerPodRunning
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      testCustomResourceName + "-" + strconv.Itoa(statefulSetNumberTwo),
			Namespace: namespace,
		},
	}

	ctx := context.Background()

	existingPVC := k8s.GetNewPVC(sampleBackupRequest, "local-home-"+instanceName+"-"+strconv.Itoa(statefulSetNumberTwo))
	err = fakeClient.Create(ctx, existingPVC)
	assert.NoError(t, err)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-pod",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "tesimage:latest",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "local-home-" + instanceName + "-" + strconv.Itoa(statefulSetNumberOne),
							MountPath: "/mnt/local",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "local-home-" + instanceName + "-" + strconv.Itoa(statefulSetNumberOne),
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/path/to/local/home",
						},
					},
				},
			},
		},
	}

	err = fakeClient.Create(ctx, pod)
	assert.NoError(t, err)

	// reconcile to assert that the pre-warmer pod cannot be created because the target PVC is being used by another pods
	res, err := r.Reconcile(ctx, req)

	assert.NoError(t, err)
	assert.Equal(t, reconcile.Result{RequeueAfter: 1 * time.Minute}, res)

	// Check that the pod was not created because the PVC is used by another pod
	createdPod := &corev1.Pod{}
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "prewarm-local-home-" + instanceName + "-" + strconv.Itoa(statefulSetNumberTwo), Namespace: namespace}, createdPod)
	assert.Error(t, err)
}
