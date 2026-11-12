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
	moncrdsv2beta2 "github.com/devgalsen/gooperatorv2/api/v2beta2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// GoFrontendReconciler reconciles a GoFrontend object
type GoFrontendReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=moncrds.senitdev.com,resources=gobackends,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=moncrds.senitdev.com,resources=gobackends/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=moncrds.senitdev.com,resources=gobackends/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=services;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
func (r *GoFrontendReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	var frontend moncrdsv2beta2.GoFrontend
	if err := r.Get(ctx, req.NamespacedName, &frontend); err != nil {
		log.Error(err, "unable to fetch GoFrontend")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	labels := map[string]string{
		"app": frontend.Name,
	}

	// 1. Réconciliation du Deployment Frontend
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      frontend.Name,
			Namespace: frontend.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		if err := ctrl.SetControllerReference(&frontend, deployment, r.Scheme); err != nil {
			return err
		}
		deployment.Spec.Replicas = &frontend.Spec.Replicas
		deployment.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: labels,
		}
		deployment.Spec.Template.ObjectMeta.Labels = labels
		deployment.Spec.Template.Spec.Containers = []corev1.Container{
			{
				Name:            "frontend",
				Image:           frontend.Spec.Image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Ports: []corev1.ContainerPort{
					{
						Name:          "http",
						ContainerPort: frontend.Spec.Port,
					},
				},
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/",
							Port: intstr.FromInt32(frontend.Spec.Port),
						},
					},
					PeriodSeconds:    10,
					FailureThreshold: 3,
				},
				Env: []corev1.EnvVar{
					{Name: "urlApi", Value: frontend.Spec.UrlApi},
				},
			},
		}
		return nil
	})
	if err != nil {
		log.Error(err, "unable to create or update Deployment")
		return ctrl.Result{}, err
	}
	log.Info("Deployment created or updated successfully")

	// 2. Réconciliation du Service Frontend
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      frontend.Name + "-service",
			Namespace: frontend.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		if err := ctrl.SetControllerReference(&frontend, service, r.Scheme); err != nil {
			return err
		}
		service.Spec.Selector = labels
		service.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "http",
				Port:       frontend.Spec.Port,
				TargetPort: intstr.FromInt(int(frontend.Spec.Port)),
			},
		}
		service.Spec.Type = corev1.ServiceTypeClusterIP
		return nil
	})
	if err != nil {
		log.Error(err, "unable to create or update Service")
		return ctrl.Result{}, err
	}
	log.Info("Service created or updated successfully")

	// 3. Réconciliation de l'Ingress Frontend
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      frontend.Name + "-ingress",
			Namespace: frontend.Namespace,
		},
	}

	if frontend.Spec.Ingress != nil {
		_, err = controllerutil.CreateOrUpdate(ctx, r.Client, ingress, func() error {
			if err := ctrl.SetControllerReference(&frontend, ingress, r.Scheme); err != nil {
				return err
			}
			if frontend.Spec.Ingress.IngressClassName != "" {
				ingress.Spec.IngressClassName = &frontend.Spec.Ingress.IngressClassName
			}

			pathType := networkingv1.PathTypePrefix
			ingress.Spec.Rules = []networkingv1.IngressRule{
				{
					Host: frontend.Spec.Ingress.Host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: service.Name,
											Port: networkingv1.ServiceBackendPort{
												Number: frontend.Spec.Port,
											},
										},
									},
								},
							},
						},
					},
				},
			}
			if frontend.Spec.Ingress.TLSSecretName != "" {
				ingress.Spec.TLS = []networkingv1.IngressTLS{
					{
						Hosts:      []string{frontend.Spec.Ingress.Host},
						SecretName: frontend.Spec.Ingress.TLSSecretName,
					},
				}
			}
			return nil
		})
		if err != nil {
			log.Error(err, "unable to create or update Ingress")
			return ctrl.Result{}, err
		}
		log.Info("Ingress reconciled successfully")
	} else {
		// Si l'ingress n'est pas spécifié, on nettoie proprement s'il existe
		if err := r.Delete(ctx, ingress); client.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to delete Ingress")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GoFrontendReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&moncrdsv2beta2.GoFrontend{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Named("gofrontend").
		Complete(r)
}
