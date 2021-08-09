/*
Copyright 2021.

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

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	vvipv1 "github.com/vviphw04/vvip-controller/api/v1"

	// APis added
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// VvipReconciler reconciles a Vvip object
type VvipReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=vvip.vviphw.io,resources=vvips,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=vvip.vviphw.io,resources=vvips/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=vvip.vviphw.io,resources=vvips/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Vvip object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *VvipReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconcile logic start.")
	// your logic here

	reqLogger := r.Log.WithValues("req.Namespace", req.Namespace, "req.Name", req.Name)
	vvip := &vvipv1.Vvip{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, vvip)

	if err != nil && errors.IsNotFound(err) {
		dep := r.deploymentForVvip(vvip)
		err = r.Client.Create(context.TODO(), dep)

		if err != nil {
			reqLogger.Error(err, "Failed to create new Deployment.", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	podList := &corev1.PodList{}
	ls := labelsForVvip(vvip.Name)
	listOps := []client.ListOption{
		client.InNamespace(req.NamespacedName.Namespace),
		client.MatchingLabels(ls),
	}
	err = r.Client.List(context.TODO(), podList, listOps...)
	if err != nil {
		reqLogger.Error(err, "Failed to list pods.", "Vvip.Namespace", vvip.Namespace, "Vvip.Name", vvip.Name)
		return ctrl.Result{}, err
	}

	podNames := getPodNames(podList.Items)

	_ = podNames

	logger.Info("Nothing happen. Please develop your reconcile logic")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VvipReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vvipv1.Vvip{}).
		Complete(r)
}

func (r *VvipReconciler) deploymentForVvip(v *vvipv1.Vvip) *appsv1.Deployment {
	ls := labelsForVvip(v.Name)
	replicas := v.Spec.Size

	dep := &appsv1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name:      v.Name,
			Namespace: v.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &v1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: v1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image:   "vvip:1.4.36-alpine",
						Name:    "vvip",
						Command: []string{"vvip", "-v=64", "-o", "modern", "-v"},
						Ports: []corev1.ContainerPort{{
							ContainerPort: 11211,
							Name:          "vvip",
						}},
					}},
				},
			},
		},
	}
	ctrl.SetControllerReference(v, dep, r.Scheme)

	return dep
}

// labelsForVvip returns the labels for selecting the resources
// belonging to the given vvip CR name.
func labelsForVvip(name string) map[string]string {
	return map[string]string{"app": "vvip", "vvip_cr": name}
}

// getPodNames returns the pod names of the array of pods passed in
func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}
