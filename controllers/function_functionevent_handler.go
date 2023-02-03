package controllers

import (
	"github.com/KubeFunction/KubeFunction/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ handler.EventHandler = &enqueueRequestForFunctionEvent{}

type enqueueRequestForFunctionEvent struct {
	reader client.Reader //readOnly[list/get]
}

func (p *enqueueRequestForFunctionEvent) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	p.addFunctionEvent(q, evt.Object)
}

func (p *enqueueRequestForFunctionEvent) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
}

func (p *enqueueRequestForFunctionEvent) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
}

func (p *enqueueRequestForFunctionEvent) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	newFunctionEvent := evt.ObjectNew.(*v1alpha1.FunctionEvent)
	oldFunctionEvent := evt.ObjectOld.(*v1alpha1.FunctionEvent)
	if newFunctionEvent.ResourceVersion == oldFunctionEvent.ResourceVersion {
		return
	}
	p.updateFunctionEvent(q, newFunctionEvent, oldFunctionEvent)
}
func (p *enqueueRequestForFunctionEvent) addFunctionEvent(q workqueue.RateLimitingInterface, obj runtime.Object) {
	functionEvent, ok := obj.(*v1alpha1.FunctionEvent)
	if !ok {
		klog.Errorf("expected FunctionEvent type")
		return
	}
	q.Add(reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      functionEvent.Spec.FunctionName,
			Namespace: functionEvent.Namespace,
		},
	})
}

func (p *enqueueRequestForFunctionEvent) updateFunctionEvent(q workqueue.RateLimitingInterface, new, old *v1alpha1.FunctionEvent) {

}
