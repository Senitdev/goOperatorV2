# 🚀 GoOperatorV2 — Opérateur Kubernetes Cloud-Native & GitOps pour Stack IA / RAG

[![Lint](https://github.com)](https://github.com)
[![Tests & Envtest](https://github.com)](https://github.com)
[![Helm Chart](https://github.com)](https://github.com)

**GoOperatorV2** est un opérateur Kubernetes développé en **Golang** avec **Kubebuilder** et **Controller-Runtime**. Il automatise de manière déclarative et autonome le cycle de vie complet (déploiement, sécurité, résilience, observabilité et mise à l'échelle) d'une architecture **IA / RAG (Retrieval-Augmented Generation)** composée d'un Frontend Next.js, d'un Backend Go/Gin et d'une base vectorielle PostgreSQL (`pgvector`).

---

## 🏗️ Architecture & Fonctionnalités de Production

L'opérateur étend l'API native de Kubernetes via trois **Custom Resource Definitions (CRDs)** sous le groupe d'API `://senitdev.com` (`GoBackend`, `GoFrontend`, `GoApi`) :

```text
                +---------------------------------------+

                |      GitOps (ArgoCD / GitHub)         |
                +-------------------+-------------------+
                                    |
                                    v
                +---------------------------------------+

                |   Custom Resources (v2beta2)          |
                |   GoBackend | GoFrontend | GoApi      |
                +-------------------+-------------------+
                                    |
                                    v
                +---------------------------------------+

                |     GoOperatorV2 (Controller)         |
                |  [Webhooks • Finalizers • Status]     |
                +---+-----------+-----------+-----------+

                    |           |           |
        +-----------+           |           +-----------+
        v                       v                       v
+---------------+      +-----------------+      +----------------+

|  Persistence  |      | Workloads/Probes|      |  Scalability   |
|  (PVC / PV)   |      | Deploy + Healthz|      |  HPA (CPU)     |
+---------------+      +--------+--------+      +----------------+
                                |
                                v
                       +-----------------+

                       | Networking & TLS|
                       | Service+Ingress |
                       +-----------------+
```

### 🛡️ Ingénierie d'Opérateur Avancée
* **Cycle de Vie & Finalizers (`://senitdev.com`) :** Interception des suppressions (`DeletionTimestamp`) et filtrage via `Predicates` pour exécuter un nettoyage propre des ressources externes avant destruction.
* **Admission Webhooks (Validation & Mutation) :**
  * *Mutating Webhook (`Default`) :* Injection dynamique de valeurs par défaut haute disponibilité (`minReplicas: 2` lorsque l'autoscaling est activé).
  * *Validating Webhook (`ValidateCreate` / `ValidateUpdate`) :* Blocage immédiat des configurations invalides dès le `kubectl apply` (cohérence `minReplicas <= maxReplicas`, validation stricte des URLs `http/https`, et protection anti-régression interdisant la réduction de taille des PVC).
* **Observabilité Native & GitOps-Ready :**
  * Gestion des **Conditions normalisées** (`Available`, `ObservedGeneration`) via `apimachinery/pkg/api/meta`.
  * Émission d'**Events Kubernetes** (`EventRecorder`) visibles via `kubectl describe` et colonnes personnalisées (`READY`, `STATUS`).

### ⚙️ Standards de Production Automatisés
* **🔐 Sécurité & RBAC :** Zéro donnée sensible en clair (`SecretKeyRef`) et permissions `ClusterRole` granulaires.
* **🩺 Haute Disponibilité :** Configuration automatique des sondes `StartupProbe`, `LivenessProbe` et `ReadinessProbe` (`/startedz`, `/healthz`, `/readyz`).
* **📈 Autoscaling Dynamique (HPA) :** Pilotage automatique des réplicas selon la charge CPU.
* **🌐 Routage Réseau (Ingress) :** Exposition externe automatisée via **Nginx Ingress**.

---

## 📋 Prérequis

* Un cluster Kubernetes fonctionnel (Docker Desktop, Kind, K3s ou cluster Cloud).
* **Nginx Ingress Controller** et **Metrics Server** actifs sur le cluster.
* **cert-manager** (requis pour le chiffrement TLS des Admission Webhooks).

---

## ⚡ Démarrage Rapide (Déploiement)

### 1. Créer le Secret de base de données
```bash
kubectl create secret generic rag-database-rag \
  --from-literal=DATABASE_URL="postgres://postgres:postgres@host.docker.internal:5437/bd_rag?sslmode=disable"
```

### 2. Installer l'Opérateur et la Stack RAG

#### Option A : Déploiement GitOps avec ArgoCD (Recommandé)
```bash
kubectl apply -f argocd/
```

#### Option B : Déploiement via Helm
```bash
helm install gooperatorv2 ./dist/chart \
  --namespace gooperatorv2-system \
  --create-namespace

kubectl apply -f config/samples/
```

---

## 🔍 Vérification & Tests Automatisés

* **Exécuter les tests d'intégration (`envtest`) et les tests de Webhooks :**
  ```bash
  make test
  make lint
  ```

* **Surveiller les ressources et les conditions de statut :**
  ```bash
  kubectl get gobackend,gofrontend
  kubectl describe gobackend rag-backend
  ```

* **Tester la montée en charge automatique (HPA) :**
  ```bash
  kubectl run -i --tty load-generator --rm --image=busybox:1.28 --restart=Never -- /bin/sh -c "while sleep 0.01; do wget -q -O- http://rag-backend:8080/healthz; done"
  ```

---

## 🧹 Nettoyage

```bash
kubectl delete -f config/samples/
helm uninstall gooperatorv2 -n gooperatorv2-system
```

---

## 👨‍💻 Auteur
Développé par **Ndiaga Toure** (`@Senitdev`) — *Platform Engineer | Développeur d'Opérateurs Kubernetes (Golang)*  
* **GitHub :(https://github.com/Senitdev/goOperatorV2)
* **LinkedIn :** [Pape Toure](www.linkedin.com/in/papa-toure-6b1287389)
