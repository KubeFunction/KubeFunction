/*
Copyright 2023 kubefunction.io.

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
	"context"
	"fmt"
	corev1alpha1 "github.com/KubeFunction/KubeFunction/api/v1alpha1"
	"github.com/KubeFunction/KubeFunction/util/controllerutils"
	"github.com/KubeFunction/KubeFunction/util/fieldindex"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// FunctionEventReconciler reconciles a FunctionEvent object
type FunctionEventReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=core.kubefunction.io,resources=functionevents,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.kubefunction.io,resources=functionevents/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.kubefunction.io,resources=functionevents/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the FunctionEvent object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *FunctionEventReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.Infof("test reconcile in function event %s", req.String())
	functionEvent := &corev1alpha1.FunctionEvent{}
	err := r.Get(ctx, req.NamespacedName, functionEvent)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		//TODO
		return reconcile.Result{}, err
	}
	if functionEvent.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
	}

	// 1.list all active and completed Pods belongs to the functionEvent
	activePods, completedPods, err := r.getOwnedPod(functionEvent)
	if err != nil {
		return reconcile.Result{}, err
	}
	// 2.scale pod
	err = r.syncFunctionEvent(activePods, completedPods, functionEvent)
	if err != nil {
		return reconcile.Result{}, err
	}
	// 3.update status

	// 4. truncate revision history

	return ctrl.Result{}, nil
}

func (r *FunctionEventReconciler) getOwnedPod(instance *corev1alpha1.FunctionEvent) ([]*v1.Pod, []*v1.Pod, error) {
	opts := &client.ListOptions{
		Namespace:     instance.Namespace,
		FieldSelector: fields.SelectorFromSet(fields.Set{fieldindex.IndexNameForOwnerRefUID: string(instance.UID)}),
	}
	podList := &v1.PodList{}
	err := r.List(context.TODO(), podList, opts)
	if err != nil {
		return nil, nil, err
	}

	var activePods []*v1.Pod
	var completedPods []*v1.Pod
	for i, pod := range podList.Items {
		if v1.PodSucceeded != pod.Status.Phase && v1.PodFailed != pod.Status.Phase && pod.DeletionTimestamp == nil {
			activePods = append(activePods, &podList.Items[i])
		} else if (v1.PodSucceeded == pod.Status.Phase || v1.PodFailed == pod.Status.Phase) && pod.DeletionTimestamp == nil {
			completedPods = append(completedPods, &podList.Items[i])
		}

	}
	return activePods, completedPods, nil
}

func (r *FunctionEventReconciler) syncFunctionEvent(activePods, completedPods []*v1.Pod, instance *corev1alpha1.FunctionEvent) error {
	if instance.Spec.Replicas == nil {
		return fmt.Errorf("functionEvent.Spec.Replicas can't be nil")
	}
	klog.Infof("len(activePods) %d,len(completedPods) %d", len(activePods), len(completedPods))
	realPodsReplicas := len(activePods) + len(completedPods)
	replicas := int(*instance.Spec.Replicas)
	// 1. user's function is existed
	if len(activePods) == 0 && realPodsReplicas == replicas {
		// this means is activePods==0 && completedPods=replicas
		return nil
	}
	// 2. is running
	if len(completedPods) == 0 && realPodsReplicas == replicas {
		return nil
	}

	// 3. nod pods belongs to th functionEvent

	// get function
	function := &corev1alpha1.Function{}
	err := r.Get(context.TODO(), client.ObjectKey{Namespace: instance.Namespace, Name: instance.Spec.FunctionName}, function)
	if err != nil {
		return err
	}
	// create pod
	podTemplateHash := controllerutils.ComputeHash(&function.Spec.Template)
	wasmPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    instance.Namespace,
			GenerateName: fmt.Sprintf("%s-%s-", instance.Name, podTemplateHash), //TODO
			Labels:       map[string]string{corev1alpha1.DefaultFunctionEventUniqueLabelKey: podTemplateHash},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(instance, corev1alpha1.GroupVersion.WithKind("FunctionEvent")),
			},
		},
		Spec: function.Spec.Template.Spec,
	}
	err = r.Create(context.TODO(), wasmPod)
	klog.V(3).Infof("pod name %s/%s: %v", wasmPod.Namespace, wasmPod.Name, err)
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *FunctionEventReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.FunctionEvent{}).
		Owns(&v1.Pod{}).
		Complete(r)
}
