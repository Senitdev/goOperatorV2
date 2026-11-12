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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// GoApiReconciler reconciles a GoApi object
type GoApiReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}
// +kubebuilder:rbac:groups=moncrds.senitdev.com,resources=gobackends,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=moncrds.senitdev.com,resources=gobackends/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=moncrds.senitdev.com,resources=gobackends/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=services;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
func (r *GoApiReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	var app moncrdsv2beta2.GoApi
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	//construire le deploiement souhaiter
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
		},
	}
	//on surveille le deploiemnt
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		if err := ctrl.SetControllerReference(
			&app, deployment, r.Scheme,
		); err != nil {
			return err
		}
		// Remplir les champs obligatoires demandés par l'API Kubernetes
		labels := map[string]string{"app": app.Name}
		deployment.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: labels,
		}
		//ficher les replicas
		replicas := app.Spec.Replicas
		if replicas == 0 {
			replicas = 1
		}
		deployment.Spec.Replicas = &replicas
		// Configurer le Template du Pod
		deployment.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
			},
			Spec: corev1.PodSpec{
				Volumes: []corev1.Volume{
					{
						Name: "storage-volume",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: app.Name + "-pvc",
							},
						},
					},
				},
				//montage du volume dans le container

				Containers: []corev1.Container{
					{
						Name:            app.Name,
						Image:           app.Spec.Image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						// // 2. Associer les volumes à des chemins (points de montage) dans le conteneur
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "storage-volume",
								MountPath: "/data/config", // Chemin dans le conteneur où le volume sera monté
							},
						},
						Ports: []corev1.ContainerPort{
							{
								Name:          "http",
								ContainerPort: app.Spec.Port,
							},
						},
					},
				},
			},
		}

		return nil
	})
	if err != nil {
		// [CORRECTION] Gestion propre des conflits causés par le rollout restart
		if errors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		log.Error(err, "Unable to reconcile Deployment")
		return ctrl.Result{}, err
	}
	//definir le service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		if err := ctrl.SetControllerReference(
			&app, service, r.Scheme,
		); err != nil {
			return err
		}
		service.Spec.Selector = map[string]string{"app": app.Name}
		service.Spec.Ports = []corev1.ServicePort{
			{
				Name: "http",
				Port: app.Spec.Port,
			},
		}
		return nil
	})
	if err != nil {
		log.Error(err, "Unable to reconcile Service")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GoApiReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&moncrdsv2beta2.GoApi{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Named("goapi").
		Complete(r)
}
