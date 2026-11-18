package v2beta2

import (
	"context"
	"testing"

	moncrdsv2beta2 "github.com/devgalsen/gooperatorv2/api/v2beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGoBackendWebhookValidation(t *testing.T) {
	ctx := context.Background()
	validator := &GoBackendValidator{}
	defaulter := &GoBackendDefaulter{}

	// 1. Tester le Mutating Webhook (Defaulting)
	backendDefault := &moncrdsv2beta2.GoBackend{
		ObjectMeta: metav1.ObjectMeta{Name: "test-default"},
		Spec: moncrdsv2beta2.GoBackendSpec{
			OllamaUrl: "http://ollama:11434",
			Autoscaling: &moncrdsv2beta2.AutoscallingConfig{
				Enabled: true,
				// On laisse MinReplicas à 0 pour vérifier qu'il passe à 2
			},
			Storage: moncrdsv2beta2.StorageConfig{Size: "1Gi"},
		},
	}

	if err := defaulter.Default(ctx, backendDefault); err != nil {
		t.Fatalf("Default() a échoué de manière inattendue : %v", err)
	}
	if backendDefault.Spec.Autoscaling.MinReplicas != 2 {
		t.Errorf("Attendu MinReplicas=2 par défaut, obtenu %d", backendDefault.Spec.Autoscaling.MinReplicas)
	} else {
		t.Log("✅ Mutation (Default) réussie : MinReplicas fixé automatiquement à 2")
	}

	// 2. Tester le rejet d'une URL Ollama invalide
	backendBadURL := backendDefault.DeepCopy()
	backendBadURL.Spec.OllamaUrl = "ftp://mauvaise-url"

	if _, err := validator.ValidateCreate(ctx, backendBadURL); err == nil {
		t.Errorf("❌ ÉCHEC : Le webhook aurait dû bloquer l'URL invalide ftp://")
	} else {
		t.Logf("✅ Blocage URL invalide réussi : %v", err)
	}

	// 3. Tester le rejet d'une inversion de réplicas (Max < Min)
	backendBadReplicas := backendDefault.DeepCopy()
	backendBadReplicas.Spec.Autoscaling.MinReplicas = 5
	backendBadReplicas.Spec.Autoscaling.MaxReplicas = 2

	if _, err := validator.ValidateCreate(ctx, backendBadReplicas); err == nil {
		t.Errorf("❌ ÉCHEC : Le webhook aurait dû bloquer MaxReplicas < MinReplicas")
	} else {
		t.Logf("✅ Blocage réplicas inversés réussi : %v", err)
	}

	// 4. Tester le blocage de la réduction de taille du stockage (Update)
	backendShrinkStorage := backendDefault.DeepCopy()
	backendShrinkStorage.Spec.Storage.Size = "500Mi" // Plus petit que 1Gi

	if _, err := validator.ValidateUpdate(ctx, backendDefault, backendShrinkStorage); err == nil {
		t.Errorf("❌ ÉCHEC : Le webhook aurait dû bloquer la réduction de taille de 1Gi à 500Mi")
	} else {
		t.Logf("✅ Blocage réduction PVC réussi : %v", err)
	}
}
