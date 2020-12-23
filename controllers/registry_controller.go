/*


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

package controllers

import (
	"context"
	"encoding/base64"
	"github.com/go-logr/logr"
	"github.com/prometheus/common/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"

	managerv1 "github.com/ycsk02/registry-controller/api/v1"
	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/json"
)

// RegistryReconciler reconciles a Registry object
type RegistryReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=manager.sukai.io,resources=registries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=manager.sukai.io,resources=registries/status,verbs=get;update;patch

func (r *RegistryReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("registry", req.NamespacedName)

	// your logic here
	reqNamespacedName := strings.Trim(req.NamespacedName.String(), "/")
	secret := api.Secret{}

	namespaceList := &api.NamespaceList{}
	if err := r.Client.List(ctx, namespaceList); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("reconcile for Registry", "name", reqNamespacedName)
	registry := managerv1.Registry{}
	if err := r.Get(ctx, client.ObjectKey{Name: reqNamespacedName}, &registry); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Registry is not found, skip", "registry", reqNamespacedName)
			return ctrl.Result{}, nil
		} else {
			log.Error(err, "failed to get Registry resource")
			return ctrl.Result{}, err
		}
	}
	if containsString(registry.Spec.NameSpace, "allnamespaces") {
		// reconcile for Registry in all namespaces
		for _, ns := range namespaceList.Items {
			if !ns.DeletionTimestamp.IsZero() {
				log.Info("delettion timestamp is not zero")
				break
			}
			expectSecret := buildSecret(registry, ns.Name)
			if err := r.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: registry.Spec.SecretName}, &secret); err != nil {
				if apierrors.IsNotFound(err) {
					log.Info("Secret is not found, creating secret", "namespace", ns.Name, "secret", expectSecret)
					if err := r.Client.Create(ctx, expectSecret); err != nil {
						return ctrl.Result{}, err
					}
				} else {
					log.Error(err, "failed to get secret resource")
					return ctrl.Result{}, err
				}
			} else {
				log.Info("secret is found in namespace", "namespace:", ns.Name, "secret:", registry.Spec.SecretName)
				if !equality.Semantic.DeepDerivative(expectSecret.Data, secret.Data) {
					if err := r.Client.Update(ctx, expectSecret); err != nil {
						log.Error(err, "failed to update secret resource")
						return ctrl.Result{}, err
					}
					log.Info("Secret is updated", "namespace", ns.Name, "secret:", registry.Spec.SecretName)
				}
			}
		}
	}
	for _, namespace := range registry.Spec.NameSpace {
		if !namespaceExists(namespaceList, namespace) {
			log.Info("namespace defined in registry is not exists, skip", "namespace", namespace)
			break
		}
		var ns api.Namespace
		if err := r.Get(context.TODO(), client.ObjectKey{Name: namespace}, &ns); err != nil {
			return ctrl.Result{}, err
		}
		if !ns.DeletionTimestamp.IsZero() {
			log.Info("delettion timestamp is not zero")
			break
		}
		expectSecret := buildSecret(registry, namespace)
		if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: registry.Spec.SecretName}, &secret); err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("Secret is not found, creating secret", "namespace", namespace, "secret", expectSecret)
				if err := r.Client.Create(ctx, expectSecret); err != nil {
					return ctrl.Result{}, err
				}
			} else {
				log.Error(err, "failed to get secret resource")
				return ctrl.Result{}, err
			}
		} else {
			log.Info("secret is found in namespace", "namespace:", namespace, "secret:", registry.Spec.SecretName)
			if !equality.Semantic.DeepDerivative(expectSecret, secret) {
				if err := r.Client.Update(ctx, expectSecret); err != nil {
					log.Error(err, "failed to update secret resource")
					return ctrl.Result{}, err
				}
				log.Info("Secret is updated", "namespace", namespace, "secret:", registry.Spec.SecretName)
			}
		}
	}
	return ctrl.Result{}, nil
}

func buildSecret(registry managerv1.Registry, namespace string) *api.Secret {
	registries := make(managerv1.RegistriesStruct)
	for _, r := range registry.Spec.RegistryServers {
		base64encode := base64.StdEncoding.EncodeToString([]byte(r.UserName + ":" + r.Password))
		cred := managerv1.RegistryCredentials{
			Username: r.UserName,
			Password: r.Password,
			Email:    r.Email,
			Auth:     base64encode,
		}
		registries[r.Server] = cred
	}

	auth := &managerv1.Auths{
		Registries: registries,
	}

	authjson, _ := json.Marshal(auth)
	secret := &api.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            registry.Spec.SecretName,
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(&registry, managerv1.GroupVersion.WithKind("Registry"))},
		},
		Type: api.SecretTypeDockerConfigJson,
		Data: map[string][]byte{".dockerconfigjson": authjson},
	}
	return secret
}

func containsString(slice []string, element string) bool {
	for _, v := range slice {
		if v == element {
			return true
		}
	}
	return false
}

func namespaceExists(namespacelist *api.NamespaceList, namespace string) bool {
	nsl := make(map[string]string)
	for _, ns := range namespacelist.Items {
		nsl[ns.Name] = ns.Name
	}
	_, ok := nsl[namespace]
	return ok
}

func (r *RegistryReconciler) NamespaceToRegistry(o handler.MapObject) []ctrl.Request {
	ctx := context.Background()
	registryList := managerv1.RegistryList{}

	// namespace := o.Meta.GetName()
	var requests []ctrl.Request
	var namespace api.Namespace

	if err := r.Get(context.TODO(), client.ObjectKey{Name: o.Meta.GetName()}, &namespace); err != nil {
		return nil
	}

	if !namespace.DeletionTimestamp.IsZero() {
		log.Info("delettion timestamp is not zero")
		return nil
	}

	if err := r.Client.List(ctx, &registryList); err != nil {
		return nil
	}

	for _, registry := range registryList.Items {
		if containsString(registry.Spec.NameSpace, "allnamespaces") || containsString(registry.Spec.NameSpace, namespace.Name) {
			requests = append(requests, ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "", Name: registry.Name}})
		}
	}

	return requests
}

func (r *RegistryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&managerv1.Registry{}).
		Owns(&api.Secret{}).
		Watches(&source.Kind{Type: &api.Namespace{}}, &handler.EnqueueRequestsFromMapFunc{ToRequests: handler.ToRequestsFunc(r.NamespaceToRegistry)}).
		Complete(r)
}
