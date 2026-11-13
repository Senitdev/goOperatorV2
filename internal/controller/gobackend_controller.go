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
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"strconv"
)

// GoBackendReconciler reconciles a GoBackend object
type GoBackendReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=moncrds.senitdev.com,resources=gobackends,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=moncrds.senitdev.com,resources=gobackends/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=moncrds.senitdev.com,resources=gobackends/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=services;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
func (r *GoBackendReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	var backend moncrdsv2beta2.GoBackend
	if err := r.Get(ctx, req.NamespacedName, &backend); err != nil {
		log.Error(err, "unable to fetch GoBackend")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	labels := map[string]string{
		"app": backend.Name,
	}
	//reconciliation automatique du pvc pour le stockage
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backend.Name + "-pvcstorage",
			Namespace: backend.Namespace,
		},
	}
	desiredSize, err := resource.ParseQuantity(backend.Spec.Storage.Size)
	if err != nil {
		log.Error(err, "Invalid PVC storage size")
		return ctrl.Result{}, err
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, pvc, func() error {
		if err := ctrl.SetControllerReference(&backend, pvc, r.Scheme); err != nil {
			return err
		}
		// Initialisation uniquement lors de la création
		if pvc.CreationTimestamp.IsZero() {
			accessMode := backend.Spec.Storage.AccessMode

			if accessMode == "" {
				accessMode = corev1.ReadWriteOnce
			}

			pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{
				accessMode,
			}
			// Gestion sécurisée de la StorageClass
			if backend.Spec.Storage.StorageClassName != nil && *backend.Spec.Storage.StorageClassName != "" {
				pvc.Spec.StorageClassName = backend.Spec.Storage.StorageClassName
			} else {
				// Optionnel : Forcer explicitement nil pour utiliser la StorageClass par défaut du cluster
				pvc.Spec.StorageClassName = nil
			}
		}
		// La capacité demandée peut être augmentée
		if pvc.Spec.Resources.Requests == nil {
			pvc.Spec.Resources.Requests = corev1.ResourceList{}
		}

		currentSize := pvc.Spec.Resources.Requests.Storage()

		if currentSize == nil || desiredSize.Cmp(*currentSize) > 0 {
			pvc.Spec.Resources.Requests[corev1.ResourceStorage] = desiredSize
		}
		return nil
	})
	if err != nil {
		log.Error(err, "unable to create or update pvc")
		return ctrl.Result{}, err
	}
	// 2.Reconciliation du Deployment Backend
	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backend.Name,
			Namespace: backend.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, &deployment, func() error {
		if err := ctrl.SetControllerReference(&backend, &deployment, r.Scheme); err != nil {
			return err
		}
		deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
		// Gestion des replicas en fonction de l'autoscaling
		if backend.Spec.Autoscaling == nil || !backend.Spec.Autoscaling.Enabled {
			deployment.Spec.Replicas = &backend.Spec.Replicas
		}
		deployment.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Labels: labels},
			Spec: corev1.PodSpec{
				Volumes: []corev1.Volume{
					{
						Name: "storage-volume",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: pvc.Name,
							},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name:            "backend",
						Image:           backend.Spec.Image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Ports: []corev1.ContainerPort{
							{Name: "http", ContainerPort: backend.Spec.Port},
						},
						//les Probes
						//Start Probes
						StartupProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/startedz",
									Port: intstr.FromInt32(backend.Spec.Port),
								},
							},
							InitialDelaySeconds: 5,
							PeriodSeconds:       5,
							FailureThreshold:    6, // Laisse 30 secondes au total à l'application pour démarrer
						},
						//LivenessProbe
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/",
									Port: intstr.FromInt32(backend.Spec.Port),
								},
							},
							PeriodSeconds:    10,
							FailureThreshold: 3,
						},
						//ReadinessProbes
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/readyz",
									Port: intstr.FromInt32(backend.Spec.Port),
								},
							},
							PeriodSeconds:    10,
							FailureThreshold: 3,
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "storage-volume", MountPath: "/app/storage/uploads"},
						},
						//gestion des ressources
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
						},
						Env: []corev1.EnvVar{
							{Name: "OLLAMA_URL", Value: backend.Spec.OllamaUrl},
							{Name: "EMBEDDING_MODEL", Value: backend.Spec.EmbeddingModel},
							{Name: "LLM_MODEL", Value: backend.Spec.LlmModel},
							{Name: "PORT", Value: strconv.FormatInt(int64(backend.Spec.Port), 10)},
							{Name: "UPLOAD_DIR", Value: "/app/storage/uploads"},
							{Name: "ALLOWED_ORIGIN", Value: backend.Spec.AllowedOrigin},
							{
								Name: "DATABASE_URL",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: backend.Spec.DatabaseRef.Name,
										},
										Key: backend.Spec.DatabaseRef.Key, // ex: "DATABASE_URL"
									},
								},
							},
						},
					},
				},
			},
		}
		return nil
	})
	if err != nil {
		if errors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		log.Error(err, "Unable to reconcile Deployment")
		return ctrl.Result{}, err
	}
	//reconcialiation du hpa
	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backend.Name + "-hpa",
			Namespace: backend.Namespace,
		},
	}
	if backend.Spec.Autoscaling != nil && backend.Spec.Autoscaling.Enabled {
		_, err = controllerutil.CreateOrUpdate(ctx, r.Client, hpa, func() error {
			if err := ctrl.SetControllerReference(&backend, hpa, r.Scheme); err != nil {
				return err
			}
			hpa.Spec.ScaleTargetRef = autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       deployment.Name,
			}
			hpa.Spec.MinReplicas = &backend.Spec.Autoscaling.MinReplicas
			hpa.Spec.MaxReplicas = backend.Spec.Autoscaling.MaxReplicas
			hpa.Spec.Metrics = []autoscalingv2.MetricSpec{
				{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: corev1.ResourceCPU,
						Target: autoscalingv2.MetricTarget{
							Type:               autoscalingv2.UtilizationMetricType,
							AverageUtilization: ptr.To(backend.Spec.Autoscaling.TargetCPUUtilizationPercentage),
						},
					},
				},
			}
			return nil
		})
		if err != nil {
			log.Error(err, "Unable to reconcile HPA")
			return ctrl.Result{}, err
		}
	} else {
		if err := r.Delete(ctx, hpa); client.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to delete HPA")
			return ctrl.Result{}, err
		}
	}
	// reconcialiation du service pour exposer le backend
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backend.Name,
			Namespace: backend.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		if err := ctrl.SetControllerReference(&backend, service, r.Scheme); err != nil {
			return err
		}
		service.Spec.Selector = labels
		service.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "http",
				Port:       backend.Spec.Port,
				TargetPort: intstr.FromInt32(backend.Spec.Port),
			},
		}
		return nil
	})
	if err != nil {
		log.Error(err, "Unable to reconcile Service")
		return ctrl.Result{}, err
	}
	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GoBackendReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&moncrdsv2beta2.GoBackend{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Named("gobackend").
		Complete(r)
}
