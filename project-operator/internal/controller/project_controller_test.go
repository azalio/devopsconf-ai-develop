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
	corev1 "k8s.io/api/core/v1"
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

	Context("Namespace attachment", func() {
		const (
			nsProjectName        = "ns-attach-project"
			projectLabelKey      = "platform.example.io/project-name"
			projectAnnotationKey = "platform.example.io/project-name"
		)

		nsProjectNN := types.NamespacedName{Name: nsProjectName}

		AfterEach(func() {
			// Strip project labels and annotations from all test namespaces
			// to prevent cross-test contamination (envtest cannot fully delete namespaces).
			nsList := &corev1.NamespaceList{}
			if err := k8sClient.List(ctx, nsList); err == nil {
				for i := range nsList.Items {
					ns := &nsList.Items[i]
					needsUpdate := false
					if ns.Labels != nil && ns.Labels[projectLabelKey] != "" {
						delete(ns.Labels, projectLabelKey)
						needsUpdate = true
					}
					if ns.Annotations != nil && ns.Annotations[projectAnnotationKey] != "" {
						delete(ns.Annotations, projectAnnotationKey)
						needsUpdate = true
					}
					if needsUpdate {
						Expect(k8sClient.Update(ctx, ns)).To(Succeed())
					}
				}
			}

			// Clean up project
			project := &platformv1alpha1.Project{}
			err := k8sClient.Get(ctx, nsProjectNN, project)
			if errors.IsNotFound(err) {
				return
			}
			Expect(err).NotTo(HaveOccurred())

			if controllerutil.ContainsFinalizer(project, projectFinalizerName) {
				controllerutil.RemoveFinalizer(project, projectFinalizerName)
				Expect(k8sClient.Update(ctx, project)).To(Succeed())
			}

			err = k8sClient.Get(ctx, nsProjectNN, project)
			if errors.IsNotFound(err) {
				return
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, project)).To(Succeed())
		})

		It("should set annotation on namespace with project label", func() {
			project := &platformv1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{Name: nsProjectName},
			}
			Expect(k8sClient.Create(ctx, project)).To(Succeed())

			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns-attach-annot",
					Labels: map[string]string{
						projectLabelKey: nsProjectName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())

			_, err := reconciler().Reconcile(ctx, reconcile.Request{
				NamespacedName: nsProjectNN,
			})
			Expect(err).NotTo(HaveOccurred())

			updated := &corev1.Namespace{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "ns-attach-annot"}, updated)).To(Succeed())
			Expect(updated.Annotations).To(HaveKeyWithValue(projectAnnotationKey, nsProjectName),
				"annotation %s must be set to project name", projectAnnotationKey)
		})

		It("should populate status.namespaces with labeled namespaces", func() {
			project := &platformv1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{Name: nsProjectName},
			}
			Expect(k8sClient.Create(ctx, project)).To(Succeed())

			for _, nsName := range []string{"ns-attach-status-1", "ns-attach-status-2"} {
				ns := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: nsName,
						Labels: map[string]string{
							projectLabelKey: nsProjectName,
						},
					},
				}
				Expect(k8sClient.Create(ctx, ns)).To(Succeed())
			}

			_, err := reconciler().Reconcile(ctx, reconcile.Request{
				NamespacedName: nsProjectNN,
			})
			Expect(err).NotTo(HaveOccurred())

			updated := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, nsProjectNN, updated)).To(Succeed())
			Expect(updated.Status.Namespaces).To(HaveLen(2),
				"status.namespaces must contain all labeled namespaces")

			nsNames := make([]string, len(updated.Status.Namespaces))
			for i, ns := range updated.Status.Namespaces {
				nsNames[i] = ns.Name
			}
			Expect(nsNames).To(ContainElements("ns-attach-status-1", "ns-attach-status-2"))
		})

		It("should include namespace phase in status.namespaces entries", func() {
			project := &platformv1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{Name: nsProjectName},
			}
			Expect(k8sClient.Create(ctx, project)).To(Succeed())

			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns-attach-phase",
					Labels: map[string]string{
						projectLabelKey: nsProjectName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())

			_, err := reconciler().Reconcile(ctx, reconcile.Request{
				NamespacedName: nsProjectNN,
			})
			Expect(err).NotTo(HaveOccurred())

			updated := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, nsProjectNN, updated)).To(Succeed())
			Expect(updated.Status.Namespaces).To(HaveLen(1))
			Expect(updated.Status.Namespaces[0].Name).To(Equal("ns-attach-phase"))
			Expect(updated.Status.Namespaces[0].Status).To(Equal("Active"),
				"namespace status must reflect the namespace phase")
		})

		It("should not include namespaces labeled for a different project", func() {
			project := &platformv1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{Name: nsProjectName},
			}
			Expect(k8sClient.Create(ctx, project)).To(Succeed())

			nsMine := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns-attach-mine",
					Labels: map[string]string{
						projectLabelKey: nsProjectName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, nsMine)).To(Succeed())

			nsOther := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns-attach-other",
					Labels: map[string]string{
						projectLabelKey: "some-other-project",
					},
				},
			}
			Expect(k8sClient.Create(ctx, nsOther)).To(Succeed())

			_, err := reconciler().Reconcile(ctx, reconcile.Request{
				NamespacedName: nsProjectNN,
			})
			Expect(err).NotTo(HaveOccurred())

			updated := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, nsProjectNN, updated)).To(Succeed())
			Expect(updated.Status.Namespaces).To(HaveLen(1),
				"status.namespaces must only contain namespaces for this project")
			Expect(updated.Status.Namespaces[0].Name).To(Equal("ns-attach-mine"))
		})

		It("should remove namespace from status when namespace is deleted", func() {
			project := &platformv1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{Name: nsProjectName},
			}
			Expect(k8sClient.Create(ctx, project)).To(Succeed())

			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns-attach-del",
					Labels: map[string]string{
						projectLabelKey: nsProjectName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())

			// First reconcile — namespace should appear in status
			r := reconciler()
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: nsProjectNN,
			})
			Expect(err).NotTo(HaveOccurred())

			first := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, nsProjectNN, first)).To(Succeed())
			Expect(first.Status.Namespaces).To(HaveLen(1),
				"namespace must appear in status after first reconcile")

			// Delete namespace
			Expect(k8sClient.Delete(ctx, ns)).To(Succeed())

			// Second reconcile — namespace must be removed from status
			_, err = r.Reconcile(ctx, reconcile.Request{
				NamespacedName: nsProjectNN,
			})
			Expect(err).NotTo(HaveOccurred())

			second := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, nsProjectNN, second)).To(Succeed())
			Expect(second.Status.Namespaces).To(BeEmpty(),
				"deleted namespace must be removed from status.namespaces")
		})

		It("should restore label if removed from managed namespace", func() {
			project := &platformv1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{Name: nsProjectName},
			}
			Expect(k8sClient.Create(ctx, project)).To(Succeed())

			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns-attach-restore",
					Labels: map[string]string{
						projectLabelKey: nsProjectName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())

			// First reconcile — sets annotation
			r := reconciler()
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: nsProjectNN,
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify annotation was set
			annotated := &corev1.Namespace{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "ns-attach-restore"}, annotated)).To(Succeed())
			Expect(annotated.Annotations).To(HaveKeyWithValue(projectAnnotationKey, nsProjectName),
				"annotation must be set after first reconcile")

			// Remove label (simulate external actor)
			delete(annotated.Labels, projectLabelKey)
			Expect(k8sClient.Update(ctx, annotated)).To(Succeed())

			// Verify label is gone
			stripped := &corev1.Namespace{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "ns-attach-restore"}, stripped)).To(Succeed())
			Expect(stripped.Labels).NotTo(HaveKey(projectLabelKey),
				"label must be removed before second reconcile")

			// Second reconcile — should restore label
			_, err = r.Reconcile(ctx, reconcile.Request{
				NamespacedName: nsProjectNN,
			})
			Expect(err).NotTo(HaveOccurred())

			restored := &corev1.Namespace{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "ns-attach-restore"}, restored)).To(Succeed())
			Expect(restored.Labels).To(HaveKeyWithValue(projectLabelKey, nsProjectName),
				"label must be restored on managed namespace (has annotation)")
		})

		It("should not duplicate namespaces in status on repeated reconciles", func() {
			project := &platformv1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{Name: nsProjectName},
			}
			Expect(k8sClient.Create(ctx, project)).To(Succeed())

			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns-attach-idem",
					Labels: map[string]string{
						projectLabelKey: nsProjectName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())

			r := reconciler()

			// First reconcile
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: nsProjectNN,
			})
			Expect(err).NotTo(HaveOccurred())

			// Second reconcile
			_, err = r.Reconcile(ctx, reconcile.Request{
				NamespacedName: nsProjectNN,
			})
			Expect(err).NotTo(HaveOccurred())

			updated := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, nsProjectNN, updated)).To(Succeed())
			Expect(updated.Status.Namespaces).To(HaveLen(1),
				"status.namespaces must not have duplicates after repeated reconcile")
		})
	})
})
