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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
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
	klog.V(3).Infof("reconcile in function event %s", req.String())
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
	err = r.updateStatus(functionEvent, activePods, completedPods)
	if err != nil {
		return reconcile.Result{}, err
	}
	// 4. truncate revision history

	return ctrl.Result{}, nil
}
func (r *FunctionEventReconciler) updateStatus(instance *corev1alpha1.FunctionEvent, activePods, completedPods []*v1.Pod) error {
	newStatus := &corev1alpha1.FunctionEventStatus{}
	r.calculateStatus(newStatus, activePods, completedPods)
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		clone := &corev1alpha1.FunctionEvent{}
		if err := r.Get(context.TODO(), types.NamespacedName{Namespace: instance.Namespace, Name: instance.Name}, clone); err != nil {
			return err
		}
		clone.Status = *newStatus
		clone.Annotations = instance.Annotations
		return r.Status().Update(context.TODO(), clone)
	})
}

func (r *FunctionEventReconciler) calculateStatus(newStatus *corev1alpha1.FunctionEventStatus, activePods, completedPods []*v1.Pod) {
	// TODO
	if len(activePods) > 0 {
		pod := activePods[0]
		newStatus.Phase = pod.Status.Phase
		return
	}
	if len(completedPods) > 0 {
		pod := completedPods[0]
		newStatus.Phase = pod.Status.Phase
		newStatus.Reason = pod.Status.ContainerStatuses[0].State.Terminated.Reason
		newStatus.ExistCode = pod.Status.ContainerStatuses[0].State.Terminated.ExitCode
		newStatus.FinishedAt = pod.Status.ContainerStatuses[0].State.Terminated.FinishedAt
		newStatus.StartTime = pod.Status.ContainerStatuses[0].State.Terminated.StartedAt
	}
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
	klog.V(3).Infof("len(activePods) %d,len(completedPods) %d", len(activePods), len(completedPods))
	realPodsReplicas := len(activePods) + len(completedPods)
	replicas := int(*instance.Spec.Replicas)
	// 1. user's function exited
	if len(activePods) == 0 && realPodsReplicas == replicas {
		// this means is activePods==0 && completedPods=replicas
		// TODO update functionEvent.spec
		return nil
	}
	// 2. is running
	if len(completedPods) == 0 && realPodsReplicas == replicas {
		return nil
	}

	// 3. no pods belong to this functionEvent

	// get function
	function := &corev1alpha1.Function{}
	err := r.Get(context.TODO(), client.ObjectKey{Namespace: instance.Namespace, Name: instance.Spec.FunctionName}, function)
	if err != nil {
		return err
	}
	// create pod
	podSpec := function.Spec.Template.Spec.DeepCopy()
	podTemplateHash := controllerutils.ComputeHash(&function.Spec.Template)
	if len(instance.Spec.Args) != 0 {
		podSpec.Containers[0].Args = instance.Spec.Args
	}
	if len(instance.Spec.Command) != 0 {
		podSpec.Containers[0].Command = instance.Spec.Command
	}
	var annotations map[string]string
	if instance.Annotations == nil {
		annotations = make(map[string]string)
	} else {
		annotations = instance.Annotations
	}
	annotations[corev1alpha1.FunctionTimeoutAnnotationKey] = string(instance.Spec.Timeout)
	wasmPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    instance.Namespace,
			GenerateName: fmt.Sprintf("%s-%s-", instance.Name, podTemplateHash),
			Labels:       map[string]string{corev1alpha1.DefaultFunctionEventUniqueLabelKey: podTemplateHash},
			Annotations:  annotations,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(instance, corev1alpha1.GroupVersion.WithKind("FunctionEvent")),
			},
		},
		Spec: *podSpec,
	}
	err = r.Create(context.TODO(), wasmPod)
	klog.V(3).Infof("pod name %s/%s: %v", wasmPod.Namespace, wasmPod.Name, err)
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *FunctionEventReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("functionevent-controller").
		For(&corev1alpha1.FunctionEvent{}).
		Owns(&v1.Pod{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(event event.CreateEvent) bool {
				klog.V(3).Infof("skip pod create event in function event reconcile loop")
				return false //skip pod create event
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				oldPod := updateEvent.ObjectOld.(*v1.Pod)
				newPod := updateEvent.ObjectNew.(*v1.Pod)
				if !newPod.GetDeletionTimestamp().IsZero() {
					return false
				}

				if oldPod.GetResourceVersion() == newPod.GetResourceVersion() {
					return false
				}

				return true
			},
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
				return true
			},
		})).
		Complete(r)
}
