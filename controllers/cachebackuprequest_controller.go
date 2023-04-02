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
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"

	cachev1beta1 "bianchi2/dc-cache-backup-operator/api/v1beta1"
)

// CacheBackupRequestReconciler reconciles a CacheBackupRequest object
type CacheBackupRequestReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	lastSpec cachev1beta1.CacheBackupRequestSpec
}

//+kubebuilder:rbac:groups=cache.atlassian.com,resources=cachebackuprequests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cache.atlassian.com,resources=cachebackuprequests/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cache.atlassian.com,resources=cachebackuprequests/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CacheBackupRequest object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *CacheBackupRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

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

	pvcs := &corev1.PersistentVolumeClaimList{}
	selector := labels.SelectorFromSet(map[string]string{
		"app.kubernetes.io/name": instance.Spec.InstanceName,
	})
	err = r.Client.List(ctx, pvcs, &client.ListOptions{
		Namespace:     instance.Namespace,
		LabelSelector: selector,
	})
	if err != nil {
		return reconcile.Result{}, err
	}

	for _, pvc := range pvcs.Items {

		if pvc.Name != "local-home-confluence-0" {
			pod := k8s.GetNewPreWarmerPod(instance, pvc.Name)
			err := r.Client.Create(ctx, pod)
			if err != nil {
				return reconcile.Result{}, err
			}

			statusChan := make(chan string)
			go func() {
				status, err := k8s.WatchPodStatus(pod.Name, pod.Namespace)
				if err != nil {
					fmt.Printf("Error watching pod status: %v\n", err)
					return
				}
				statusChan <- status
			}()

			select {
			case status := <-statusChan:
				fmt.Printf("Pod %s status: %s\n", pod.Name, status)
				currentTime := time.Now()

				instance.Status.PVCStatus = []cachev1beta1.PVCStatus{
					{
						PVCName:             pvc.Name,
						Status:              status,
						LastTransactionTime: currentTime.Format("2006-01-02 15:04:05"),
					},
				}
				err := r.Client.Status().Update(ctx, instance)
				if err != nil {
					return reconcile.Result{}, err
				}
			case <-time.After(10 * time.Minute):
				fmt.Printf("Timed out waiting for pod status\n")
			}
		}
	}

	return ctrl.Result{RequeueAfter: 10 * time.Minute}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CacheBackupRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1beta1.CacheBackupRequest{}).
		Complete(r)
}
