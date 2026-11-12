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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	moncrdsv2beta2 "github.com/devgalsen/gooperatorv2/api/v2beta2"
)

var _ = Describe("GoBackend Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName      = "test-backend"
			resourceNamespace = "default"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}
		gobackend := &moncrdsv2beta2.GoBackend{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind GoBackend")
			err := k8sClient.Get(ctx, typeNamespacedName, gobackend)
			if err != nil && errors.IsNotFound(err) {
				resource := &moncrdsv2beta2.GoBackend{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
					Spec: moncrdsv2beta2.GoBackendSpec{
						Image:    "my-rag-backend:latest",
						Port:     8080,
						Replicas: 1,
						//DatabaseUrl: "postgres://postgres:postgres@postgres:5432/bd_rag?sslmode=disable",
						OllamaUrl: "http://docker.internal",

						Storage: moncrdsv2beta2.StorageConfig{
							Size:       "5Gi",
							AccessMode: corev1.ReadWriteOnce,
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &moncrdsv2beta2.GoBackend{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance GoBackend")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &GoBackendReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("1. Verifying that the automatic PVC was successfully created")
			createdPvc := &corev1.PersistentVolumeClaim{}
			pvcLookupKey := types.NamespacedName{Name: resourceName + "-pvc2", Namespace: resourceNamespace}
			err = k8sClient.Get(ctx, pvcLookupKey, createdPvc)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdPvc.Spec.AccessModes).To(ContainElement(corev1.ReadWriteOnce))

			By("2. Verifying that the Deployment was successfully created with accurate spec")
			createdDeployment := &appsv1.Deployment{}
			err = k8sClient.Get(ctx, typeNamespacedName, createdDeployment)
			Expect(err).NotTo(HaveOccurred())
			Expect(*createdDeployment.Spec.Replicas).To(Equal(int32(1)))
			Expect(createdDeployment.Spec.Template.Spec.Containers).To(HaveLen(1))

			By("Checking backend container environment variables configuration")
			containerEnv := createdDeployment.Spec.Template.Spec.Containers[0].Env
			Expect(containerEnv).To(ContainElement(corev1.EnvVar{Name: "OLLAMA_URL", Value: "http://docker.internal"}))
			Expect(containerEnv).To(ContainElement(corev1.EnvVar{Name: "DATABASE_URL", Value: "postgres://postgres:postgres@postgres:5432/bd_rag?sslmode=disable"}))

			By("3. Verifying that the Service was successfully created")
			createdService := &corev1.Service{}
			err = k8sClient.Get(ctx, typeNamespacedName, createdService)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdService.Spec.Ports[0].Port).To(Equal(int32(8080)))

			By("4. Checking OwnerReferences to ensure clean Garbage Collection")
			Expect(createdDeployment.GetOwnerReferences()).To(HaveLen(1))
			Expect(createdDeployment.GetOwnerReferences()[0].Name).To(Equal(resourceName))
			Expect(createdPvc.GetOwnerReferences()).To(HaveLen(1))
			Expect(createdPvc.GetOwnerReferences()[0].Name).To(Equal(resourceName))
		})
	})
})
