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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	platformv1alpha1 "github.com/example/project-operator/api/v1alpha1"
)

var _ = Describe("Project Controller", func() {
	const projectName = "test-project"

	ctx := context.Background()

	namespacedName := types.NamespacedName{
		Name: projectName,
	}

	reconciler := func() *ProjectReconciler {
		return &ProjectReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}
	}

	AfterEach(func() {
		project := &platformv1alpha1.Project{}
		err := k8sClient.Get(ctx, namespacedName, project)
		if errors.IsNotFound(err) {
			return
		}
		Expect(err).NotTo(HaveOccurred())

		// Remove finalizer if present so the object can be deleted
		if controllerutil.ContainsFinalizer(project, projectFinalizerName) {
			controllerutil.RemoveFinalizer(project, projectFinalizerName)
			Expect(k8sClient.Update(ctx, project)).To(Succeed())
		}

		// If the object had a deletionTimestamp, removing the finalizer
		// causes Kubernetes to auto-delete it.
		err = k8sClient.Get(ctx, namespacedName, project)
		if errors.IsNotFound(err) {
			return
		}
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Delete(ctx, project)).To(Succeed())
	})

	Context("When a new Project is created", func() {
		It("should add the project-protection finalizer", func() {
			project := &platformv1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: projectName,
				},
			}
			Expect(k8sClient.Create(ctx, project)).To(Succeed())

			_, err := reconciler().Reconcile(ctx, reconcile.Request{
				NamespacedName: namespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			updated := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, namespacedName, updated)).To(Succeed())
			Expect(controllerutil.ContainsFinalizer(updated, projectFinalizerName)).
				To(BeTrue(), "finalizer %s must be present after reconcile", projectFinalizerName)
		})

		It("should set status.phase to Active", func() {
			project := &platformv1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: projectName,
				},
			}
			Expect(k8sClient.Create(ctx, project)).To(Succeed())

			_, err := reconciler().Reconcile(ctx, reconcile.Request{
				NamespacedName: namespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			updated := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, namespacedName, updated)).To(Succeed())
			Expect(updated.Status.Phase).To(Equal(platformv1alpha1.ProjectPhaseActive),
				"status.phase must be Active after reconcile")
		})
	})

	Context("When a Project has deletionTimestamp set", func() {
		It("should set status.phase to Terminating", func() {
			project := &platformv1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:       projectName,
					Finalizers: []string{projectFinalizerName},
				},
			}
			Expect(k8sClient.Create(ctx, project)).To(Succeed())

			// Delete sets deletionTimestamp but doesn't remove the object because of the finalizer
			Expect(k8sClient.Delete(ctx, project)).To(Succeed())

			// Verify deletionTimestamp is set
			deleted := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, namespacedName, deleted)).To(Succeed())
			Expect(deleted.DeletionTimestamp).NotTo(BeNil(), "deletionTimestamp must be set after Delete")

			_, err := reconciler().Reconcile(ctx, reconcile.Request{
				NamespacedName: namespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			updated := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, namespacedName, updated)).To(Succeed())
			Expect(updated.Status.Phase).To(Equal(platformv1alpha1.ProjectPhaseTerminating),
				"status.phase must be Terminating when deletionTimestamp is set")
		})
	})

	Context("Idempotency", func() {
		It("should produce the same result when reconciled multiple times", func() {
			project := &platformv1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: projectName,
				},
			}
			Expect(k8sClient.Create(ctx, project)).To(Succeed())

			r := reconciler()

			// First reconcile
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: namespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			first := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, namespacedName, first)).To(Succeed())

			// Second reconcile
			_, err = r.Reconcile(ctx, reconcile.Request{
				NamespacedName: namespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			second := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, namespacedName, second)).To(Succeed())

			// State must be identical
			Expect(second.Status.Phase).To(Equal(first.Status.Phase),
				"status.phase must be the same after repeated reconcile")
			Expect(controllerutil.ContainsFinalizer(second, projectFinalizerName)).
				To(BeTrue(), "finalizer must still be present after repeated reconcile")
			Expect(second.Finalizers).To(HaveLen(len(first.Finalizers)),
				"finalizer must not be duplicated after repeated reconcile")
		})

		It("should not error when the Project does not exist", func() {
			_, err := reconciler().Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "nonexistent-project"},
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
