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
	registryList := managerv1.RegistryList{}
	if err := r.Client.List(ctx, &registryList); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Did not find any Registry")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	namespaceList := &api.NamespaceList{}
	if err := r.Client.List(ctx, namespaceList); err != nil {
		return ctrl.Result{}, err
	}

	if namespaceExists(namespaceList, reqNamespacedName) {
		// reconcile for Namespace watch
		log.Info("reconcile for Namespace")
		for _, registry := range registryList.Items {
			if containsString(registry.Spec.NameSpace, "allnamespaces") || containsString(registry.Spec.NameSpace, reqNamespacedName) {
				// this registry is for allnamespaces or my namespace
				expectSecret := buildSecret(registry, reqNamespacedName)
				if err := r.Client.Get(ctx, client.ObjectKey{Namespace: reqNamespacedName, Name: registry.Spec.SecretName}, &secret); err != nil {
					if apierrors.IsNotFound(err) {
						log.Info("Secret is not found, creating secret", "namespace", reqNamespacedName, "secret", expectSecret)
						if err := r.Client.Create(ctx, expectSecret); err != nil {
							return ctrl.Result{}, err
						}
					} else {
						log.Error(err, "failed to get secret resource")
						return ctrl.Result{}, err
					}
				} else {
					log.Info("secret is found in namespace, update it if not the newest", "namespace:", reqNamespacedName, "secret:", registry.Spec.SecretName)
					if !equality.Semantic.DeepDerivative(expectSecret, secret) {
						if err := r.Client.Update(ctx, expectSecret); err != nil {
							log.Error(err, "failed to update secret resource")
							return ctrl.Result{}, err
						}
						log.Info("Secret is updated", "namespace", reqNamespacedName, "secret:", registry.Spec.SecretName)
					}
				}

			} else {
				// this namespace is not in any registries, ignore it
				log.Info("This namespace is not in any registries", "namespace", reqNamespacedName)
			}
		}

	} else {
		// reconcile for Registry watch
		log.Info("reconcile for Registry")
		registry := managerv1.Registry{}
		if err := r.Client.Get(ctx, client.ObjectKey{Name: reqNamespacedName}, &registry); err != nil {
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
				expectSecret := buildSecret(registry, ns.Name)
				if err := r.Client.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: registry.Spec.SecretName}, &secret); err != nil {
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
					log.Info("secret is found in namespace, update it if not the newest", "namespace:", ns.Name, "secret:", registry.Spec.SecretName)
					if !equality.Semantic.DeepDerivative(expectSecret, secret) {
						if err := r.Client.Update(ctx, expectSecret); err != nil {
							log.Error(err, "failed to update secret resource")
							return ctrl.Result{}, err
						}
						log.Info("Secret is updated", "namespace", ns.Name, "secret:", registry.Spec.SecretName)
					}
				}
			}
		} else {
			// reconcile for Registry in my namespaces
			for _, namespace := range registry.Spec.NameSpace {
				if !namespaceExists(namespaceList, namespace) {
					log.Info("namespace defined in registry is not exists, skip", "namespace", namespace)
					break
				}
				expectSecret := buildSecret(registry, namespace)
				if err := r.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: registry.Spec.SecretName}, &secret); err != nil {
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
					log.Info("secret is found in namespace, update it if not the newest", "namespace:", namespace, "secret:", registry.Spec.SecretName)
					if !equality.Semantic.DeepDerivative(expectSecret, secret) {
						if err := r.Client.Update(ctx, expectSecret); err != nil {
							log.Error(err, "failed to update secret resource")
							return ctrl.Result{}, err
						}
						log.Info("Secret is updated", "namespace", namespace, "secret:", registry.Spec.SecretName)
					}
				}
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

func (r *RegistryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&managerv1.Registry{}).
		Watches(&source.Kind{Type: &api.Namespace{}}, &handler.EnqueueRequestForObject{}).
		Watches(&source.Kind{Type: &managerv1.Registry{}}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
