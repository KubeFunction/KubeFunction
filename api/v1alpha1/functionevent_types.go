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

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultFunctionEventUniqueLabelKey string = "pod-template-hash"
	FunctionTimeoutAnnotationKey       string = "core.kubefunction.io/function-timeout"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FunctionEventSpec defines the desired state of FunctionEvent
type FunctionEventSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	FunctionName string   `json:"functionName,omitempty"`
	Args         []string `json:"args,omitempty"`
	Command      []string `json:"command,omitempty"`
	Replicas     *int32   `json:"replicas,omitempty"`
	Timeout      int32    `json:"timeout,omitempty"`
}

// FunctionEventStatus defines the observed state of FunctionEvent
type FunctionEventStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Phase      v1.PodPhase `json:"phase,omitempty"`
	FinishedAt metav1.Time `json:"finishedAt,omitempty"`
	ExistCode  int32       `json:"existCode,omitempty"`
	Reason     string      `json:"reason,omitempty"`
	StartTime  metav1.Time `json:"startTime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// FunctionEvent is the Schema for the functionevents API
type FunctionEvent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FunctionEventSpec   `json:"spec,omitempty"`
	Status FunctionEventStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FunctionEventList contains a list of FunctionEvent
type FunctionEventList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FunctionEvent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FunctionEvent{}, &FunctionEventList{})
}
