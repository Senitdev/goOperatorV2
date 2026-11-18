package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
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
			secretName        = "test-db-secret"
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
						Image:     "my-rag-backend:latest",
						Port:      8080,
						Replicas:  1,
						OllamaUrl: "http://docker.internal",
						DatabaseRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
							Key:                  "DATABASE_URL",
						},
						Autoscaling: &moncrdsv2beta2.AutoscallingConfig{
							Enabled:                        true,
							MinReplicas:                    2,
							MaxReplicas:                    5,
							TargetCPUUtilizationPercentage: 80,
						},
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
			if err == nil {
				By("Cleanup the specific resource instance GoBackend")
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

				// Exécuter Reconcile pour traiter le Finalizer et autoriser la suppression
				controllerReconciler := &GoBackendReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}
				_, _ = controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
			}
		})

		It("should successfully reconcile the resource", func() {
			controllerReconciler := &GoBackendReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			By("1st Reconcile: Adding the Finalizer")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("2nd Reconcile: Creating PVC, Deployment, HPA and Service")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("1. Verifying that the automatic PVC was successfully created")
			createdPvc := &corev1.PersistentVolumeClaim{}
			pvcLookupKey := types.NamespacedName{Name: resourceName + "-pvcstorage", Namespace: resourceNamespace}
			Expect(k8sClient.Get(ctx, pvcLookupKey, createdPvc)).To(Succeed())
			Expect(createdPvc.Spec.AccessModes).To(ContainElement(corev1.ReadWriteOnce))

			By("2. Verifying that the Deployment was successfully created")
			createdDeployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, createdDeployment)).To(Succeed())
			Expect(createdDeployment.Spec.Template.Spec.Containers).To(HaveLen(1))

			By("Checking SecretKeyRef injection for DATABASE_URL")
			containerEnv := createdDeployment.Spec.Template.Spec.Containers[0].Env
			Expect(containerEnv).To(ContainElement(corev1.EnvVar{
				Name: "DATABASE_URL",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
						Key:                  "DATABASE_URL",
					},
				},
			}))

			By("3. Verifying that the HPA was successfully created")
			createdHpa := &autoscalingv2.HorizontalPodAutoscaler{}
			hpaLookupKey := types.NamespacedName{Name: resourceName + "-hpa", Namespace: resourceNamespace}
			Expect(k8sClient.Get(ctx, hpaLookupKey, createdHpa)).To(Succeed())
			Expect(createdHpa.Spec.MaxReplicas).To(Equal(int32(5)))

			By("4. Checking OwnerReferences to ensure clean Garbage Collection")
			Expect(createdDeployment.GetOwnerReferences()).To(HaveLen(1))
			Expect(createdDeployment.GetOwnerReferences()[0].Name).To(Equal(resourceName))
			Expect(createdPvc.GetOwnerReferences()).To(HaveLen(1))
			Expect(createdPvc.GetOwnerReferences()[0].Name).To(Equal(resourceName))
		})
	})
})
