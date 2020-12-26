/*
Copyright 2020.

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
	managerv1 "github.com/ycsk02/registry-controller/api/v1"
	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
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
	var (
		registry      managerv1.Registry
		secret        api.Secret
		namespace     api.Namespace
		namespaceList api.NamespaceList
		conditions    []managerv1.Condition
	)

	if err := r.Get(ctx, client.ObjectKey{Name: req.Name}, &registry); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	conditions = registry.Status.Conditions

	switch {
	case containsString(registry.Spec.TargetNamespace, "allnamespaces"):
		if err := r.List(ctx, &namespaceList); err != nil {
			return ctrl.Result{}, err
		}

		for _, ns := range namespaceList.Items {
			if !ns.GetDeletionTimestamp().IsZero() {
				r.Log.Info("namespace is terminating, ignore", "namespace", ns.Name)
				continue
			}

			expectSecret := buildSecret(registry, ns.Name)

			if err := r.Get(ctx, client.ObjectKey{Name: registry.Spec.SecretName, Namespace: ns.Name}, &secret); err != nil {
				if !apierrors.IsNotFound(err) {
					return ctrl.Result{}, err
				}

				log.Info("create secret in namespace", "namespace", ns.Name)
				if err := r.Create(ctx, expectSecret); err != nil {
					c := managerv1.Condition{
						Type:               "CreateFailed",
						Status:             api.ConditionStatus("False"),
						NamespaceResult:    []string{ns.Name},
						LastTransitionTime: metav1.Now(),
					}
					conditions = append(conditions, c)
					_ = r.Status().Update(ctx, &registry)
					return ctrl.Result{}, err
				}
				c := managerv1.Condition{
					Type:               "CreateSuccessful",
					Status:             api.ConditionStatus("True"),
					NamespaceResult:    []string{ns.Name},
					LastTransitionTime: metav1.Now(),
				}
				conditions = append(conditions, c)
			} else {
				if equality.Semantic.DeepDerivative(expectSecret.Data, secret.Data) {
					continue
				}
				if err := r.Update(ctx, expectSecret); err != nil {
					c := managerv1.Condition{
						Type:               "UpdateFailed",
						Status:             api.ConditionStatus("False"),
						NamespaceResult:    []string{ns.Name},
						LastTransitionTime: metav1.Now(),
					}
					conditions = append(conditions, c)
					_ = r.Status().Update(ctx, &registry)
					return ctrl.Result{}, err
				}
				c := managerv1.Condition{
					Type:               "UpdateSuccessful",
					Status:             api.ConditionStatus("True"),
					NamespaceResult:    []string{ns.Name},
					LastTransitionTime: metav1.Now(),
				}
				conditions = append(conditions, c)
			}
		}
	default:
		for _, ns := range registry.Spec.TargetNamespace {
			if err := r.Get(ctx, client.ObjectKey{Name: ns}, &namespace); err != nil {
				if apierrors.IsNotFound(err) {
					log.Info("namespace is not found skip", "namespace", ns)
					c := managerv1.Condition{
						Type:               "NamespaceNotFound",
						Status:             api.ConditionStatus("False"),
						NamespaceResult:    []string{ns},
						LastTransitionTime: metav1.Now(),
					}
					conditions = append(conditions, c)
					_ = r.Status().Update(ctx, &registry)
					continue
				} else {
					return ctrl.Result{}, err
				}
			}

			if !namespace.GetDeletionTimestamp().IsZero() {
				r.Log.Info("namespace is terminating, ignore", "namespace", namespace.Name)
				continue
			}

			expectSecret := buildSecret(registry, ns)

			if err := r.Get(ctx, client.ObjectKey{Name: registry.Spec.SecretName, Namespace: ns}, &secret); err != nil {
				if !apierrors.IsNotFound(err) {
					return ctrl.Result{}, err
				}

				log.Info("create secret in namespace", "namespace", ns)
				if err := r.Create(ctx, expectSecret); err != nil {
					c := managerv1.Condition{
						Type:               "CreateFailed",
						Status:             api.ConditionStatus("False"),
						NamespaceResult:    []string{ns},
						LastTransitionTime: metav1.Now(),
					}
					conditions = append(conditions, c)
					_ = r.Status().Update(ctx, &registry)
					return ctrl.Result{}, err
				}
				c := managerv1.Condition{
					Type:               "CreateSuccessful",
					Status:             api.ConditionStatus("True"),
					NamespaceResult:    []string{ns},
					LastTransitionTime: metav1.Now(),
				}
				conditions = append(conditions, c)
			} else {
				if equality.Semantic.DeepDerivative(expectSecret.Data, secret.Data) {
					continue
				}
				log.Info("update secret in namespace", "namespace", ns)
				if err := r.Update(ctx, expectSecret); err != nil {
					c := managerv1.Condition{
						Type:               "UpdateFailed",
						Status:             api.ConditionStatus("False"),
						NamespaceResult:    []string{ns},
						LastTransitionTime: metav1.Now(),
					}
					conditions = append(conditions, c)
					return ctrl.Result{}, err
				}
				c := managerv1.Condition{
					Type:               "UpdateSuccessful",
					Status:             api.ConditionStatus("True"),
					NamespaceResult:    []string{ns},
					LastTransitionTime: metav1.Now(),
				}
				conditions = append(conditions, c)
			}
		}
	}
	registry.Status.Conditions = conditions
	_ = r.Status().Update(ctx, &registry)
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

func (r *RegistryReconciler) NamespaceToRegistry(o handler.MapObject) []ctrl.Request {
	var (
		requests     []ctrl.Request
		registryList managerv1.RegistryList
	)

	if err := r.List(context.TODO(), &registryList); err != nil {
		return nil
	}

	for _, reg := range registryList.Items {
		if containsString(reg.Spec.TargetNamespace, "allnamespaces") ||
			containsString(reg.Spec.TargetNamespace, o.Meta.GetName()) {
			requests = append(requests, ctrl.Request{NamespacedName: types.NamespacedName{Name: reg.Name}})
		}
	}

	return requests
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func (r *RegistryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&managerv1.Registry{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(createEvent event.CreateEvent) bool {
				return true
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				if updateEvent.MetaNew.GetGeneration() == updateEvent.MetaOld.GetGeneration() {
					return false
				} else {
					return true
				}
			},
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
				return true
			},
		})).
		Owns(&api.Secret{}).
		Watches(
			&source.Kind{Type: &api.Namespace{}},
			&handler.EnqueueRequestsFromMapFunc{ToRequests: handler.ToRequestsFunc(r.NamespaceToRegistry)},
			builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(createEvent event.CreateEvent) bool {
					return true
				},
				UpdateFunc: func(updateEvent event.UpdateEvent) bool {
					return false
				},
				DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
					return false
				},
		})).
		Complete(r)
}
