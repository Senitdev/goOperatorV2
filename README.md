# 🚀 GoOperatorV2 — Kubernetes Operator pour Stack IA / RAG

[![Go Version](https://shields.io)](https://golang.org)
[![Kubernetes](https://shields.io)](https://kubernetes.io)
[![Kubebuilder](https://shields.io)](https://kubebuilder.io)
[![License](https://shields.io)](https://opensource.org)

**GoOperatorV2** est un opérateur Kubernetes développé en Go avec **Kubebuilder** et **Controller-Runtime**. Il automatise le déploiement, la sécurisation, la résilience et la mise à l'échelle (autoscaling) d'une architecture complète d'application **RAG (Retrieval-Augmented Generation)** composée d'un Frontend Next.js, d'un Backend Go/Gin et d'une base vectorielle PostgreSQL (`pgvector`).

---

## 🏗️ Architecture & Fonctionnalités de Production

Cet opérateur implémente les meilleures pratiques de production Cloud-Native à travers trois Custom Resource Definitions (**CRD**) sous le groupe d'API `://senitdev.com` (`GoBackend`, `GoFrontend`) :

                        +-----------------------------------+
                        |   Custom Resources (v2beta2)      |
                        |      GoBackend | GoFrontend   |
                        +-----------------+-----------------+
                                          |
                                          v
                        +------------------------------------+
                        |     GoOperatorV2 (Controller)      |
                        +--+-------+-------+--------+------+
                        |       |       |        |
                        +---------------+-------|-------|+---------------+-------|
                        v                       v       v                        v
                        +---------------+      +--------+-------+-------+       +----------------+
                        |  Persistence  |      |  Workloads & Probes    |       |  Scalability   |
                        |  (PVC / PV)   |      | Deployment + Healthz   |       |  HPA (CPU)     |
                        +---------------+      +--------+---------------+       +----------------+
                                        |
                                        v
                        +--------+---------------+
                        |   Networking & TLS     |
                        |   Service + Ingress    |
                        +------------------------+

### Points clés de l'implémentation :
* **🔐 Sécurité & Gestion des Secrets :** Aucune donnée sensible en clair. Les chaînes de connexion PostgreSQL sont injectées dynamiquement dans les conteneurs via `SecretKeyRef`.

* **🩺 Haute Disponibilité & Résilience :** Configuration automatique des sondes Kubernetes (`StartupProbe`, `LivenessProbe`, `ReadinessProbe`) sur les endpoints dédiés (`/startedz`, `/healthz`, `/readyz`) pour garantir un routage sans coupure.
* **📈 Autoscaling Dynamique (HPA) :** Gestion déclarative des objets `HorizontalPodAutoscaler` (`autoscaling/v2`) avec définition stricte des `Requests` et `Limits` CPU/RAM pour absorber les pics de charge lors de l'ingestion de documents.
* **🌐 Routage Réseau Automatisé (Ingress) :** Création et nettoyage conditionnel des règles `networking.k8s.io/v1` (Nginx Ingress) avec support optionnel des certificats TLS.
* **💾 Stockage Persistant Sécurisé :** Provisionnement automatique des `PersistentVolumeClaim` avec protection contre le redimensionnement destructif à la baisse.
* **🛡️ Principe du Moindre Privilège (RBAC) :** Permissions `ClusterRole` granulaires générées pour le `ServiceAccount` du gestionnaire.

---

## 📋 Prérequis

* Un cluster Kubernetes fonctionnel (Docker Desktop, Kind, Minikube ou cluster Cloud).
* `kubectl` configuré pour communiquer avec le cluster.
* **Nginx Ingress Controller** installé (pour l'exposition externe).
* **Metrics Server** actif sur le cluster (requis pour le fonctionnement du HPA).

---

## ⚡ Démarrage Rapide (Déploiement dans le Cluster)

### 1. Créer le Secret de base de données
Avant de déployer le backend, créez le secret contenant l'URL de connexion à votre instance PostgreSQL (`pgvector`) :

```bash
kubectl create secret generic rag-database-rag \
  --from-literal=DATABASE_URL="postgres://postgres:postgres@host.docker.internal:5437/bd_rag?sslmode=disable"
```
### 2. Installer l'Opérateur (Choisissez une option)

#### Option A : Déploiement via Helm (Recommandé pour la Production)
Générez le Chart Helm officiel depuis les manifestes Kubebuilder et déployez l'opérateur en une seule commande :

```bash
# 1. Générer le Chart Helm dans dist/chart
kubebuilder edit --plugins=helm/v2-alpha

# 2. Packager le Chart (Optionnel pour la distribution)
helm package dist/chart

# 3. Installer l'opérateur dans le cluster
helm install gooperatorv2 ./dist/chart \
  --namespace gooperatorv2-system \
  --create-namespace \
  --set manager.image.repository=gooperatorv2 \
  --set manager.image.tag=v1.0.0
```

#### Option B : Déploiement direct via Makefile (Développement)
```bash
# Construire l'image localement
make docker-build IMG=gooperatorv2:v1.0.1

# Déployer le gestionnaire dans le cluster
make deploy IMG=gooperatorv2:v1.0.1
```

Vérifiez que l'opérateur est bien opérationnel :
```bash
kubectl get pods -n gooperatorv2-system
```

---
### 3. Déployer l'Opérateur
Installez les CRD, les règles RBAC et le contrôleur dans le namespace dédié `gooperatorv2-system` :
```bash
# Construire l'image localement
make docker-build IMG=gooperatorv2:v1.0.0
# Déployer le gestionnaire dans le cluster
make deploy IMG=gooperatorv2:v1.0.0
```
Vérifiez que l'opérateur est bien opérationnel :
```bash
kubectl get pods -n gooperatorv2-system
```

### 4. Déployer la Stack Applicative (Backend & Frontend)
Appliquez les manifestes d'exemple pour déclencher la boucle de réconciliation :

```bash
kubectl apply -f config/samples/moncrds_v2beta2_gobackend.yaml
kubectl apply -f config/samples/moncrds_v2beta2_gofrontend.yaml
```

---

## 🔍 Vérification & Tests en Conditions Réelles

* **Surveiller les ressources générées :**
  ```bash
  kubectl get deploy,svc,pvc,ingress,hpa
  ```
* **Tester l'autoscaling (HPA) :**
  Générez une charge CPU sur le service Backend pour observer la montée automatique des réplicas :
  ```bash
  kubectl run -i --tty load-generator --rm --image=busybox:1.28 --restart=Never -- /bin/sh -c "while sleep 0.01; do wget -q -O- http://rag-backend:8080/healthz; done"
  ```
* **Accéder à l'interface Web :**
  Ouvrez votre navigateur sur `http://rag.local` (si configuré dans votre ressource `GoFrontend`).
---
## 🧹 Nettoyage

Pour supprimer les applications et désinstaller proprement l'opérateur du cluster :

```bash
kubectl delete -f config/samples/
make undeploy
```
---

## 👨‍💻 Auteur
Développé par **Paco** (`@devgalsen`) — Projet d'ingénierie Cloud Native & Kubernetes.
