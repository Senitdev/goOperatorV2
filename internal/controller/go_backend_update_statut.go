/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://apache.org

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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// updateStatus met à jour le sous-statut ReadyReplicas, les Conditions normalisées et émet les Events
func (r *GoBackendReconciler) UpdateStatus(ctx context.Context, backend *moncrdsv2beta2.GoBackend, deployment *appsv1.Deployment) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var currentDeploy appsv1.Deployment
	if err := r.Get(ctx, client.ObjectKeyFromObject(deployment), &currentDeploy); err != nil {
		log.Error(err, "Impossible de lire l'état du Deployment pour le statut")
		return ctrl.Result{}, err
	}

	backend.Status.ReadyReplicas = currentDeploy.Status.ReadyReplicas

	if currentDeploy.Status.ReadyReplicas > 0 && currentDeploy.Status.ReadyReplicas == currentDeploy.Status.Replicas {
		meta.SetStatusCondition(&backend.Status.Conditions, metav1.Condition{
			Type:               "Available",
			Status:             metav1.ConditionTrue,
			Reason:             "ReconciliationSucceeded",
			Message:            "Le backend RAG, le stockage et le réseau sont opérationnels.",
			ObservedGeneration: backend.Generation,
		})
		if r.Recorder != nil {
			r.Recorder.Eventf(backend, nil, corev1.EventTypeNormal, "Ready", "Reconcile", "Toutes les ressources GoBackend sont prêtes")
		}
	} else {
		meta.SetStatusCondition(&backend.Status.Conditions, metav1.Condition{
			Type:               "Available",
			Status:             metav1.ConditionFalse,
			Reason:             "PodsNotReady",
			Message:            "En attente du démarrage complet des Pods backend.",
			ObservedGeneration: backend.Generation,
		})
	}

	if err := r.Status().Update(ctx, backend); err != nil {
		log.Error(err, "Échec de la mise à jour du statut GoBackend")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
