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

package v2beta2

import (
	"context"
	moncrdsv2beta2 "github.com/devgalsen/gooperatorv2/api/v2beta2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"strings"
)

// nolint:unused
// log is for logging in this package.
var gobackendlog = logf.Log.WithName("gobackend-resource")

// SetupGoBackendWebhookWithManager registers the webhook for GoBackend in the manager.
func SetupGoBackendWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &moncrdsv2beta2.GoBackend{}).
		WithValidator(&GoBackendValidator{}).
		WithDefaulter(&GoBackendDefaulter{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-moncrds-senitdev-com-v2beta2-gobackend,mutating=true,failurePolicy=fail,sideEffects=None,groups=moncrds.senitdev.com,resources=gobackends,verbs=create;update,versions=v2beta2,name=mgobackend-v2beta2.kb.io,admissionReviewVersions=v1

// GoBackendDefaulter struct is responsible for setting default values on the custom resource of the
// Kind GoBackend when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type GoBackendDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

// Default implements admission.Defaulter so a webhook will be registered for the Kind GoBackend.
func (d *GoBackendDefaulter) Default(_ context.Context, obj *moncrdsv2beta2.GoBackend) error {
	gobackendlog.Info("Defaulting for GoBackend", "name", obj.GetName())
	//si l'autoscalling est active sans MinReplicas definis ,forcer 2 replicas pour la haute
	//disponibilite
	if obj.Spec.Autoscaling != nil && obj.Spec.Autoscaling.Enabled {
		if obj.Spec.Autoscaling.MinReplicas == 0 {
			obj.Spec.Autoscaling.MinReplicas = 2
		}
		if obj.Spec.Autoscaling.MaxReplicas == 0 {
			obj.Spec.Autoscaling.MaxReplicas = 5
		}
	} else if obj.Spec.Replicas == 0 {
		//Sinon garantir au moins 1 replicas par defaut
		obj.Spec.Replicas = 1
	}
	// TODO(user): fill in your defaulting logic.

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: If you want to customise the 'path', use the flags '--defaulting-path' or '--validation-path'.
// +kubebuilder:webhook:path=/validate-moncrds-senitdev-com-v2beta2-gobackend,mutating=false,failurePolicy=fail,sideEffects=None,groups=moncrds.senitdev.com,resources=gobackends,verbs=create;update,versions=v2beta2,name=vgobackend-v2beta2.kb.io,admissionReviewVersions=v1

// GoBackendValidator struct is responsible for validating the GoBackend resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type GoBackendValidator struct {
	// TODO(user): Add more fields as needed for validation
}

// ValidateCreate implements admission.Validator so a webhook will be registered for the type GoBackend.
func (v *GoBackendValidator) ValidateCreate(_ context.Context, obj *moncrdsv2beta2.GoBackend) (admission.Warnings, error) {
	gobackendlog.Info("Validation for GoBackend upon creation", "name", obj.GetName())

	// TODO(user): fill in your validation logic upon object creation.
	gobackendlog.Info("Validation for GoBackend upon creation", "name", obj.GetName())
	return nil, v.validateGoBackend(obj)
}

// ValidateUpdate implements admission.Validator so a webhook will be registered for the type GoBackend.
func (v *GoBackendValidator) ValidateUpdate(_ context.Context, oldObj, newObj *moncrdsv2beta2.GoBackend) (admission.Warnings, error) {
	gobackendlog.Info("Validation for GoBackend upon update", "name", newObj.GetName())
	var allErrs field.ErrorList
	//1.Appliquer les variables de bases
	if err := v.validateGoBackend(newObj); err != nil {
		return nil, err
	}
	//2.Interdire la diminution de la taille du stockage PVC
	oldSize, errOld := resource.ParseQuantity(oldObj.Spec.Storage.Size)
	newSize, errNew := resource.ParseQuantity(newObj.Spec.Storage.Size)
	if errOld == nil && errNew == nil && newSize.Cmp(oldSize) < 0 {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("storage").Child("size"),
			newObj.Spec.Storage.Size,
			"impossible de réduire la taille du stockage existant (doit être supérieure ou égale à "+oldObj.Spec.Storage.Size+")",
		))

	}
	if len(allErrs) > 0 {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "moncrds.senitdev.com", Kind: "GoBackend"},
			newObj.Name,
			allErrs,
		)
	}
	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements admission.Validator so a webhook will be registered for the type GoBackend.
func (v *GoBackendValidator) ValidateDelete(_ context.Context, obj *moncrdsv2beta2.GoBackend) (admission.Warnings, error) {
	gobackendlog.Info("Validation for GoBackend upon deletion", "name", obj.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}

// validateGoBackend regroupe les règles métier communes à la création et à la mise à jour
func (v *GoBackendValidator) validateGoBackend(obj *moncrdsv2beta2.GoBackend) error {
	var allErrs field.ErrorList
	// Règle 1 : Vérifier que l'URL d'Ollama est valide
	if !strings.HasPrefix(obj.Spec.OllamaUrl, "http://") && !strings.HasPrefix(obj.Spec.OllamaUrl, "https://") {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("ollamaUrl"),
			obj.Spec.OllamaUrl,
			"l'URL d'Ollama doit impérativement commencer par http:// ou https://",
		))
	}

	// Règle 2 : Vérifier la cohérence des bornes d'autoscaling (HPA)
	if obj.Spec.Autoscaling != nil && obj.Spec.Autoscaling.Enabled {
		if obj.Spec.Autoscaling.MaxReplicas < obj.Spec.Autoscaling.MinReplicas {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec").Child("autoscaling").Child("maxReplicas"),
				obj.Spec.Autoscaling.MaxReplicas,
				"maxReplicas ne peut pas être inférieur à minReplicas",
			))
		}
	}

	// Règle 3 : Vérifier que la quantité de stockage est syntaxiquement valide
	if _, err := resource.ParseQuantity(obj.Spec.Storage.Size); err != nil {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("storage").Child("size"),
			obj.Spec.Storage.Size,
			"format de taille invalide (exemples valides : 1Gi, 500Mi)",
		))
	}

	if len(allErrs) > 0 {
		return apierrors.NewInvalid(
			schema.GroupKind{Group: "moncrds.senitdev.com", Kind: "GoBackend"},
			obj.Name,
			allErrs,
		)
	}

	return nil
}
