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
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProjectRoleSpec defines the desired state of ProjectRole.
type ProjectRoleSpec struct {
	// Rules holds the RBAC policy rules for this role.
	// +kubebuilder:validation:MinItems=1
	Rules []rbacv1.PolicyRule `json:"rules"`
}

// ProjectRoleStatus defines the observed state of ProjectRole.
type ProjectRoleStatus struct {
	// Conditions represent the latest available observations of the role's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ProjectRole is the Schema for the projectroles API.
// It defines a custom role that can be referenced by ProjectAccessBinding.
type ProjectRole struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProjectRoleSpec   `json:"spec"`
	Status ProjectRoleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProjectRoleList contains a list of ProjectRole.
type ProjectRoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectRole `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ProjectRole{}, &ProjectRoleList{})
}
