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
	"github.com/KubeFunction/KubeFunction/api/v1alpha1"
	"github.com/KubeFunction/KubeFunction/util/fieldindex"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

func init() {
	scheme = runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)
}

var (
	scheme            *runtime.Scheme
	replicas          = int32(1)
	functionEventDemo = v1alpha1.FunctionEvent{
		ObjectMeta: metav1.ObjectMeta{Name: "fe-test-0", Namespace: "default", UID: types.UID("fe-test-0")},
		Spec: v1alpha1.FunctionEventSpec{
			FunctionName: functionDemo.Name,
			Replicas:     &replicas,
		},
	}
	runtimeClassName = "wasmrun"
	functionDemo     = v1alpha1.Function{
		ObjectMeta: metav1.ObjectMeta{Name: "func-test-0", Namespace: "default", UID: types.UID("func-test-0")},
		Spec: v1alpha1.FunctionSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					RuntimeClassName: &runtimeClassName,
					Containers: []v1.Container{
						{
							Name:  "test",
							Image: "docker.io/duizhang/hello-wasm",
						},
					},
				},
			},
		},
	}
)

func TestReconcile(t *testing.T) {
	cases := []struct {
		name       string
		req        ctrl.Request
		expectPods []*v1.Pod
	}{
		{
			name: "test-owned-pod",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      functionEventDemo.Name,
					Namespace: functionEventDemo.Namespace,
				},
			},
			expectPods: []*v1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("test-%s", functionEventDemo.Name),
						Namespace: functionEventDemo.Namespace,
						OwnerReferences: []metav1.OwnerReference{
							{UID: functionEventDemo.UID, Name: functionEventDemo.Name},
						},
					},
					Spec: v1.PodSpec{
						RuntimeClassName: &runtimeClassName,
					},
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithIndex(&v1.Pod{}, fieldindex.IndexNameForOwnerRefUID, fieldindex.OwnerIndexFunc).Build()

	err := fakeClient.Create(context.TODO(), &functionDemo)
	if err != nil {
		t.Fatalf("create function demo error %v", err)
	}
	err = fakeClient.Create(context.TODO(), &functionEventDemo)
	if err != nil {
		t.Fatalf("create functionevent demo error %v", err)
	}
	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			recon := FunctionEventReconciler{Client: fakeClient}
			_, err := recon.Reconcile(context.TODO(), cs.req)
			if err != nil {
				t.Fatalf("Reconcile failed: %s", err.Error())
			}
			if !checkPodEqual(fakeClient, t, cs.expectPods) {
				t.Fatalf("Reconcile failed [pod is not expected]")
			}
		})
	}

}

func checkPodEqual(c client.WithWatch, t *testing.T, expect []*v1.Pod) bool {
	for i := range expect {
		obj := expect[i]
		pod := &v1.Pod{}
		err := c.Get(context.TODO(), client.ObjectKey{Namespace: obj.Namespace, Name: obj.Name}, pod)
		if err != nil {
			t.Fatalf("get pod failed %v", err)
			return false
		}
	}

	return true
}

func checkStatus(c client.WithWatch, t *testing.T, expect []*v1.Pod) bool {
	return true
}
