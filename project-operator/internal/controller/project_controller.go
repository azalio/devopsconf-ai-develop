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

package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	platformv1alpha1 "github.com/example/project-operator/api/v1alpha1"
)

const (
	projectFinalizerName = "platform.example.io/project-protection"
	projectLabelKey      = "platform.example.io/project-name"
	projectAnnotationKey = "platform.example.io/project-name"
)

// ProjectReconciler reconciles a Project object
type ProjectReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=platform.example.io,resources=projects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=platform.example.io,resources=projects/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=platform.example.io,resources=projects/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;update;patch

// Reconcile moves the cluster state closer to the desired state for a Project.
func (r *ProjectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	project := &platformv1alpha1.Project{}
	if err := r.Get(ctx, req.NamespacedName, project); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle deletion: set phase to Terminating
	if !project.DeletionTimestamp.IsZero() {
		log.Info("Project is being deleted", "project", project.Name)
		if project.Status.Phase != platformv1alpha1.ProjectPhaseTerminating {
			project.Status.Phase = platformv1alpha1.ProjectPhaseTerminating
			if err := r.Status().Update(ctx, project); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(project, projectFinalizerName) {
		controllerutil.AddFinalizer(project, projectFinalizerName)
		if err := r.Update(ctx, project); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Reconcile namespaces: set annotations, restore labels, build status list
	if err := r.reconcileNamespaces(ctx, project); err != nil {
		return ctrl.Result{}, err
	}

	// Set phase to Active
	if project.Status.Phase != platformv1alpha1.ProjectPhaseActive {
		project.Status.Phase = platformv1alpha1.ProjectPhaseActive
	}

	// Persist status (phase + namespaces)
	if err := r.Status().Update(ctx, project); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileNamespaces finds namespaces belonging to this project,
// sets annotations, restores missing labels, and updates status.namespaces.
func (r *ProjectReconciler) reconcileNamespaces(ctx context.Context, project *platformv1alpha1.Project) error {
	log := logf.FromContext(ctx)
	projectName := project.Name

	// Find namespaces labeled for this project
	var labeledList corev1.NamespaceList
	if err := r.List(ctx, &labeledList, client.MatchingLabels{
		projectLabelKey: projectName,
	}); err != nil {
		return err
	}

	var nsStatuses []platformv1alpha1.NamespaceStatus
	labeledNames := make(map[string]struct{})

	for i := range labeledList.Items {
		ns := &labeledList.Items[i]
		if !ns.DeletionTimestamp.IsZero() {
			continue
		}

		labeledNames[ns.Name] = struct{}{}

		// Set annotation if missing or incorrect
		if ns.Annotations == nil {
			ns.Annotations = make(map[string]string)
		}
		if ns.Annotations[projectAnnotationKey] != projectName {
			ns.Annotations[projectAnnotationKey] = projectName
			if err := r.Update(ctx, ns); err != nil {
				return err
			}
		}

		nsStatuses = append(nsStatuses, platformv1alpha1.NamespaceStatus{
			Name:   ns.Name,
			Status: string(ns.Status.Phase),
		})
	}

	// Find managed namespaces that lost their label (have annotation but no label)
	var allList corev1.NamespaceList
	if err := r.List(ctx, &allList); err != nil {
		return err
	}

	for i := range allList.Items {
		ns := &allList.Items[i]
		if !ns.DeletionTimestamp.IsZero() {
			continue
		}
		if _, ok := labeledNames[ns.Name]; ok {
			continue
		}
		if ns.Annotations == nil || ns.Annotations[projectAnnotationKey] != projectName {
			continue
		}

		// Managed namespace lost its label — restore it
		if ns.Labels == nil {
			ns.Labels = make(map[string]string)
		}
		ns.Labels[projectLabelKey] = projectName
		if err := r.Update(ctx, ns); err != nil {
			return err
		}
		log.Info("Restored project label on managed namespace", "namespace", ns.Name, "project", projectName)

		nsStatuses = append(nsStatuses, platformv1alpha1.NamespaceStatus{
			Name:   ns.Name,
			Status: string(ns.Status.Phase),
		})
	}

	project.Status.Namespaces = nsStatuses
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.Project{}).
		Watches(&corev1.Namespace{}, handler.EnqueueRequestsFromMapFunc(
			r.namespaceToProject,
		)).
		Named("project").
		Complete(r)
}

// namespaceToProject maps a Namespace event to the owning Project reconcile request.
func (r *ProjectReconciler) namespaceToProject(ctx context.Context, obj client.Object) []reconcile.Request {
	ns, ok := obj.(*corev1.Namespace)
	if !ok {
		return nil
	}

	if projectName, ok := ns.Labels[projectLabelKey]; ok && projectName != "" {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: projectName}}}
	}

	if projectName, ok := ns.Annotations[projectAnnotationKey]; ok && projectName != "" {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: projectName}}}
	}

	return nil
}
