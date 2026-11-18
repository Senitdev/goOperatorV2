package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	moncrdsv2beta2 "github.com/devgalsen/gooperatorv2/api/v2beta2"
)

var _ = Describe("GoFrontend Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName      = "test-frontend"
			resourceNamespace = "default"
			ingressHost       = "rag-test.127.0.0.1.nip.io"
		)
		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}
		gofrontend := &moncrdsv2beta2.GoFrontend{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind GoFrontend")
			err := k8sClient.Get(ctx, typeNamespacedName, gofrontend)
			if err != nil && errors.IsNotFound(err) {
				resource := &moncrdsv2beta2.GoFrontend{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
					Spec: moncrdsv2beta2.GoFrontendSpec{
						Replicas: 2,
						Image:    "my-rag-frontend:latest",
						Port:     3000,
						UrlApi:   "http://rag-backend:8080",
						Ingress: &moncrdsv2beta2.IngressConfig{
							Enable:           true,
							Host:             ingressHost,
							IngressClassName: "nginx",
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &moncrdsv2beta2.GoFrontend{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				By("Cleanup the specific resource instance GoFrontend")
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should successfully reconcile the Deployment, Service, and Ingress", func() {
			By("Reconciling the created GoFrontend resource")
			controllerReconciler := &GoFrontendReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("1. Verifying that the Deployment was created with accurate spec")
			createdDeployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, createdDeployment)).To(Succeed())
			Expect(*createdDeployment.Spec.Replicas).To(Equal(int32(2)))
			Expect(createdDeployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(createdDeployment.Spec.Template.Spec.Containers[0].Image).To(Equal("my-rag-frontend:latest"))

			By("2. Verifying that the Service was created")
			createdService := &corev1.Service{}
			serviceLookupKey := types.NamespacedName{Name: resourceName + "-service", Namespace: resourceNamespace}
			Expect(k8sClient.Get(ctx, serviceLookupKey, createdService)).To(Succeed())
			Expect(createdService.Spec.Ports[0].Port).To(Equal(int32(3000)))

			By("3. Verifying that the Ingress was created when enabled")
			createdIngress := &networkingv1.Ingress{}
			ingressLookupKey := types.NamespacedName{Name: resourceName + "-ingress", Namespace: resourceNamespace}
			Expect(k8sClient.Get(ctx, ingressLookupKey, createdIngress)).To(Succeed())
			Expect(createdIngress.Spec.Rules).To(HaveLen(1))
			Expect(createdIngress.Spec.Rules[0].Host).To(Equal(ingressHost))

			By("4. Checking OwnerReferences for cascading deletion")
			Expect(createdDeployment.GetOwnerReferences()).To(HaveLen(1))
			Expect(createdDeployment.GetOwnerReferences()[0].Name).To(Equal(resourceName))
			Expect(createdIngress.GetOwnerReferences()).To(HaveLen(1))
			Expect(createdIngress.GetOwnerReferences()[0].Name).To(Equal(resourceName))
		})
	})
})
