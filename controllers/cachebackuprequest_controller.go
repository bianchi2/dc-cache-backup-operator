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
	cachev1beta1 "bianchi2/dc-cache-backup-operator/api/v1beta1"
	"context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strconv"
	"time"
)

const dateFormatLayout = "2006-01-02 15:04:05 -0700"

type TestSuite struct {
	Test  bool
	State string
}

// CacheBackupRequestReconciler reconciles a CacheBackupRequest object
type CacheBackupRequestReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	K8sClient kubernetes.Interface
	Test      TestSuite
}

//+kubebuilder:rbac:groups=cache.atlassian.com,resources=cachebackuprequests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cache.atlassian.com,resources=cachebackuprequests/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cache.atlassian.com,resources=cachebackuprequests/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *CacheBackupRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	log := log.FromContext(ctx)

	instance := &cachev1beta1.CacheBackupRequest{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if len(instance.Status.LastTransactionTime) > 0 {
		runBackup, err := isBackupOutdated(instance)
		if err != nil || !runBackup {
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}
	}

	// check if pvc exists and is available
	pvcName := "local-home-" + instance.Spec.InstanceName + "-" + strconv.Itoa(instance.Spec.StatefulSetNumber)
	exists, free, err := IsPVCExistsAndFree(instance, pvcName, r.K8sClient)

	// create PVC if missing
	if !exists {
		if instance.Spec.CreatePVC {
			log.Info("PVC " + pvcName + " does not exist. Creating it because .spec.createPVC is " + strconv.FormatBool(instance.Spec.CreatePVC))
			pvc := GetNewPVC(instance, pvcName)
			err = r.Client.Create(ctx, pvc)
			if err != nil && !errors.IsAlreadyExists(err) {
				return reconcile.Result{RequeueAfter: 1 * time.Minute}, err
			}
		} else {
			log.Error(err, "PVC does not exist")
			crStatus := &cachev1beta1.CacheBackupRequestStatus{
				PVCName:                     pvcName,
				Status:                      "PVCDoesNotExist",
				LastTransactionTime:         time.Now().Format(dateFormatLayout),
				IndexRestoreDurationSeconds: 0,
			}
			err := r.UpdateStatus(ctx, req, crStatus)
			if err != nil {
				return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil
			}
			return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil
		}
	}

	if !free {
		// this isn't really a reconciliation error but rather one of the expected scenarios
		// so we requeue and try again later
		if instance.Status.Status == string(corev1.PodRunning) || instance.Status.Status == string(corev1.PodPending) {
		} else {
			log.Info("PVC " + pvcName + " is bound to PV that is currently used by a running pod. Waiting 1 minute...")
			return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil

		}
	}

	pod := GetNewPreWarmerPod(instance, pvcName)
	err = r.Client.Create(ctx, pod)
	if err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	}

	// create a channel for receiving pod status updates
	statusChan := make(chan string)

	go func() {
		status, err := WatchPodStatus(pod.Name, pod.Namespace, r.K8sClient, r.Test.State)
		if err != nil {
			log.Error(err, "Error watching pod status")
			return
		}
		statusChan <- status
	}()

	select {
	case status := <-statusChan:
		_ = r.Client.Get(ctx, req.NamespacedName, instance)

		// skip updating the same status
		if status != instance.Status.Status {
			log.Info("Pod " + pod.Name + " status changed to " + status)
			log.Info("Updating " + instance.Name + " status from " + instance.Status.Status + " to " + status)

			currentTime := time.Now()
			indexRestoreDuration := 0
			crStatus := &cachev1beta1.CacheBackupRequestStatus{
				PVCName:                     pvcName,
				Status:                      status,
				LastTransactionTime:         currentTime.Format(dateFormatLayout),
				IndexRestoreDurationSeconds: indexRestoreDuration,
			}

			// update custom resource status
			err := r.UpdateStatus(ctx, req, crStatus)
			if err != nil {
				return reconcile.Result{}, err
			}
		}

		// we don't need a pod that has succeeded, so deleting it
		// keeping failed pods will result in no more pre-warmer pods being created
		// until the faulty pod is manually deleted (after examining logs)
		if status == string(corev1.PodSucceeded) {
			pod := r.GetRuntimePreWarmerPod(pod)
			indexRestoreDuration := int(time.Since(pod.ObjectMeta.CreationTimestamp.Time).Seconds())
			currentTime := time.Now()
			// update custom resource status
			crStatus := &cachev1beta1.CacheBackupRequestStatus{
				PVCName:                     pvcName,
				Status:                      status,
				LastTransactionTime:         currentTime.Format(dateFormatLayout),
				IndexRestoreDurationSeconds: indexRestoreDuration,
			}

			err := r.UpdateStatus(ctx, req, crStatus)
			if err != nil {
				return reconcile.Result{RequeueAfter: 1 * time.Second}, err
			}

			log.Info("Deleting pod " + pod.Name)
			err = r.Client.Delete(ctx, pod)
			if err != nil {
				return ctrl.Result{RequeueAfter: 1 * time.Second}, err
			}
			return ctrl.Result{RequeueAfter: time.Duration(instance.Spec.BackupIntervalMinutes) * time.Minute}, nil
		}

	// this shouldn't happen - a pod should have at least some status
	// unless pod creation ended with failure
	case <-time.After(5 * time.Minute):
		log.Info("Timed out waiting for pod status")
	}
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

func (r *CacheBackupRequestReconciler) UpdateStatus(ctx context.Context, req ctrl.Request, status *cachev1beta1.CacheBackupRequestStatus) (err error) {
	instance := &cachev1beta1.CacheBackupRequest{}

	var updateErr error
	for i := 0; i < 5; i++ {
		err = r.Client.Get(ctx, req.NamespacedName, instance)
		instance.Status = *status
		if err != nil {
			return err
		}
		updateErr = r.Client.Status().Update(ctx, instance)
		if updateErr == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return updateErr
}

func isBackupOutdated(cr *cachev1beta1.CacheBackupRequest) (outdated bool, err error) {

	layout := dateFormatLayout
	lastTransactionTimeStr := cr.Status.LastTransactionTime
	lastTransactionTime, err := time.Parse(layout, lastTransactionTimeStr)
	if err != nil {
		return false, err
	}
	interval := time.Duration(cr.Spec.BackupIntervalMinutes) * time.Minute
	currentTime := time.Now()

	if cr.Status.Status == "Succeeded" && currentTime.Sub(lastTransactionTime) < (interval) {
		return false, nil
	}
	return true, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CacheBackupRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1beta1.CacheBackupRequest{}).
		Complete(r)
}
