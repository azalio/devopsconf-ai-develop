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

// ProjectReference identifies a Project by name.
type ProjectReference struct {
	// Name of the Project.
	Name string `json:"name"`
}

// RoleReference identifies a ProjectRole by kind and name.
type RoleReference struct {
	// Kind is always "ProjectRole".
	// +kubebuilder:validation:Enum=ProjectRole
	Kind string `json:"kind"`

	// Name of the ProjectRole.
	Name string `json:"name"`
}

// ProjectAccessBindingSpec defines the desired state of ProjectAccessBinding.
// +kubebuilder:validation:XValidation:rule="has(self.role) || has(self.roleRef)",message="either role or roleRef must be specified"
// +kubebuilder:validation:XValidation:rule="!(has(self.role) && has(self.roleRef))",message="role and roleRef are mutually exclusive"
type ProjectAccessBindingSpec struct {
	// ProjectRef references the project this binding applies to.
	ProjectRef ProjectReference `json:"projectRef"`

	// Role is a built-in role name (project.admin, project.developer, project.viewer).
	// Mutually exclusive with RoleRef.
	// +kubebuilder:validation:Enum="project.admin";"project.developer";"project.viewer"
	// +optional
	Role string `json:"role,omitempty"`

	// RoleRef references a custom ProjectRole.
	// Mutually exclusive with Role.
	// +optional
	RoleRef *RoleReference `json:"roleRef,omitempty"`

	// Subjects holds references to the users, groups, or service accounts this binding grants access to.
	// +kubebuilder:validation:MinItems=1
	Subjects []rbacv1.Subject `json:"subjects"`
}

// ProjectAccessBindingStatus defines the observed state of ProjectAccessBinding.
type ProjectAccessBindingStatus struct {
	// Conditions represent the latest available observations of the binding's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Project",type=string,JSONPath=`.spec.projectRef.name`
// +kubebuilder:printcolumn:name="Role",type=string,JSONPath=`.spec.role`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ProjectAccessBinding is the Schema for the projectaccessbindings API.
// It grants subjects access to a project using a built-in or custom role.
type ProjectAccessBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProjectAccessBindingSpec   `json:"spec"`
	Status ProjectAccessBindingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProjectAccessBindingList contains a list of ProjectAccessBinding.
type ProjectAccessBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectAccessBinding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ProjectAccessBinding{}, &ProjectAccessBindingList{})
}
