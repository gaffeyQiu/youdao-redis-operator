/*
Copyright 2022.

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
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/pointer"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	redisv1beta1 "github.com/gaffeyQiu/youdao-redis-operator/api/v1beta1"
)

// RedisClusterReconciler reconciles a RedisCluster object
type RedisClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=redis.io,resources=redisclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=redis.io,resources=redisclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=redis.io,resources=redisclusters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the RedisCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *RedisClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)
	l.Info("Reconciling")

	instance := &redisv1beta1.RedisCluster{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	err = r.CreateRedisStatefulSet(instance, "leader")
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.CreateRedisStatefulSet(instance, "slave")
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *RedisClusterReconciler) CreateRedisStatefulSet(cr *redisv1beta1.RedisCluster, redisType string) error {
	replicas := *cr.Spec.Size
	if redisType == "leader" {
		replicas = 1
	} else {
		replicas -= 1
	}

	stateful := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        cr.ObjectMeta.Name + "-" + redisType,
			Namespace:   cr.Namespace,
			Labels:      getRedisLabels(cr.Name, redisType, cr.Labels),
			Annotations: getAnots(cr.Annotations),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: pointer.Int32Ptr(replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": cr.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      getRedisLabels(cr.Name, redisType, cr.Labels),
					Annotations: getAnots(cr.Annotations),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "redis" + redisType,
							Image: "redis:latest",
						},
					},
				},
			},
		},
	}

	create, err := generateK8SClient().AppsV1().StatefulSets(cr.Namespace).Create(context.TODO(), stateful, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	fmt.Println("success", create.Name)

	return nil
}

func generateK8SClient() *kubernetes.Clientset {
	restCfg, err := clientcmd.BuildConfigFromFlags("", "/Users/gaffey/.kube/config")
	if err != nil {
		panic(err)
	}
	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		panic(err)
	}
	return clientset
}

func getRedisLabels(name string, redisType string, lbs map[string]string) map[string]string {
	if lbs == nil {
		lbs = make(map[string]string)
	}
	lbs["app"] = name
	lbs["role"] = redisType
	return lbs
}

func getAnots(anots map[string]string) map[string]string {
	if anots == nil {
		anots = make(map[string]string)
	}
	anots["redis.op"] = "true"
	return anots
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redisv1beta1.RedisCluster{}).
		Complete(r)
}
