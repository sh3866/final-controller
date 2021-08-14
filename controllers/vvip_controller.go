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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vvipv1 "github.com/vviphw04/vvip-controller/api/v1"

	// APis added
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// VvipReconciler reconciles a Vvip object
type VvipReconciler struct {
	client.Client
	Log logr.Logger
	*runtime.Scheme
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
func (r *VvipReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	reqLogger := r.Log.WithValues("req.Namespace", req.Namespace, "req.Name", req.Name)
	reqLogger.Info("Reconciling Memcached.")
	// your logic here

	vvip := &vvipv1.Vvip{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, vvip)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile req.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("Vvip resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the req.
		reqLogger.Error(err, "Failed to get Memcached.")
		return ctrl.Result{}, err
	}

	deployment := &appsv1.Deployment{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: vvip.Name, Namespace: vvip.Namespace}, deployment)

	if err != nil && errors.IsNotFound(err) {
		dep := r.deploymentForVvip(vvip)
		err = r.Client.Create(context.TODO(), dep)

		if err != nil {
			reqLogger.Error(err, "Failed to create new Deployment.", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Deployment.")
		return ctrl.Result{}, err
	}

	// Ensure the deployment size is the same as the spec
	size := vvip.Spec.Size
	if *deployment.Spec.Replicas != size {
		deployment.Spec.Replicas = &size
		err = r.Client.Update(context.TODO(), deployment)
		if err != nil {
			reqLogger.Error(err, "Failed to update Deployment.", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
			return ctrl.Result{}, err
		}
	}

	// Update the Memcached status with the pod names
	// List the pods for this memcached's deployment
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

	if !reflect.DeepEqual(podNames, vvip.Status.Nodes) {
		vvip.Status.Nodes = podNames
		err := r.Client.Status().Update(context.TODO(), vvip)
		if err != nil {
			reqLogger.Error(err, "Failed to update Memcached status.")
			return ctrl.Result{}, err
		}
	}

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

// serviceForMemcached function takes in a Memcached object and returns a Service for that object.
func (r *VvipReconciler) serviceForMemcached(v *vvipv1.Vvip) *corev1.Service {
	ls := labelsForVvip(v.Name)
	ser := &corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name:      v.Name,
			Namespace: v.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: ls,
			Ports: []corev1.ServicePort{
				{
					Port: 11211,
					Name: v.Name,
				},
			},
		},
	}
	// Set Memcached instance as the owner of the Service.
	ctrl.SetControllerReference(v, ser, r.Scheme) //todo check how to get the schema
	return ser
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
