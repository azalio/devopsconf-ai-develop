/*
Copyright 2026.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProjectPhase describes the lifecycle phase of a Project.
// +kubebuilder:validation:Enum=Active;Terminating
type ProjectPhase string

const (
	ProjectPhaseActive      ProjectPhase = "Active"
	ProjectPhaseTerminating ProjectPhase = "Terminating"
)

// ProjectSpec defines the desired state of Project.
type ProjectSpec struct {
	// DisplayName is a human-readable name for the project.
	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// Description is a human-readable description of the project.
	// +optional
	Description string `json:"description,omitempty"`

	// Quotas defines resource limits for the project across all its namespaces.
	// Keys: requests.cpu, requests.memory, limits.cpu, limits.memory.
	// +optional
	Quotas corev1.ResourceList `json:"quotas,omitempty"`
}

// NamespaceStatus describes the state of a namespace within a project.
type NamespaceStatus struct {
	// Name of the namespace.
	Name string `json:"name"`

	// Status of the namespace (e.g. Active, Terminating).
	// +optional
	Status string `json:"status,omitempty"`
}

// ProjectStatus defines the observed state of Project.
type ProjectStatus struct {
	// Phase is the current lifecycle phase of the project.
	// +optional
	Phase ProjectPhase `json:"phase,omitempty"`

	// Namespaces lists namespaces belonging to this project.
	// +optional
	Namespaces []NamespaceStatus `json:"namespaces,omitempty"`

	// UsedQuotas reflects the actual resource consumption across all project namespaces.
	// +optional
	UsedQuotas corev1.ResourceList `json:"usedQuotas,omitempty"`

	// Conditions represent the latest available observations of the project's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Namespaces",type=string,JSONPath=`.status.namespaces[*].name`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Project is the Schema for the projects API.
// It groups namespaces into a logical unit with shared quotas and access control.
type Project struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProjectSpec   `json:"spec,omitempty"`
	Status ProjectStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProjectList contains a list of Project.
type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Project `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Project{}, &ProjectList{})
}
