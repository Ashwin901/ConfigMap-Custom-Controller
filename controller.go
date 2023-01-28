package main

import (
	"context"
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	coreInformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	coreListers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// We can add the original config map as the parent ref, so that whenever it is deleted all the other cm's should also be deleted

type controller struct {
	clientset kubernetes.Interface            // used to interact with the kubernetes api server
	lister    coreListers.ConfigMapLister     // used to get the resources from the cache
	hasSynced cache.InformerSynced            // used to check if resources are initialised in the cache
	queue     workqueue.RateLimitingInterface // used to add resources in the queue, which require processing
}

func newController(clientset kubernetes.Interface, congifmapInformer coreInformers.ConfigMapInformer) *controller {

	controller := controller{
		clientset: clientset,
		lister:    congifmapInformer.Lister(),
		hasSynced: congifmapInformer.Informer().HasSynced,
		queue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "configmapCustomController"),
	}

	// adding event handlers for certain events
	congifmapInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.handleAdd,
			DeleteFunc: controller.handleDelete,
		},
	)

	return &controller
}

func (c *controller) Run(ch <-chan struct{}) {

	fmt.Println("Start controller")

	if !cache.WaitForCacheSync(ch, c.hasSynced) {
		fmt.Println("Caches not synced")
	}

	// we can have multiple workers if required, in this case we are just using one worker
	go wait.Until(c.worker, 1*time.Second, ch)

	<-ch // blocking operation
}

func (c *controller) worker() {
	for c.processQueue() {

	}
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

func (c *controller) processQueue() bool {

	// namespaces that we will not be considering
	ignoredNamespaces := []string{"default", "kube-system", "kube-public", "kube-node-lease", "local-path-storage"}

	item, shutdown := c.queue.Get()

	if shutdown {
		return false
	}

	defer c.queue.Done(item)

	ns, name, err := cache.SplitMetaNamespaceKey(item.(string))

	if err != nil {
		fmt.Println("Invalid key")
		c.queue.Forget(item) // we forget the item because the key is invalid and not use of requeuing
		return false
	}

	// ignore config maps created in specific namespaces
	if contains(ignoredNamespaces, ns) {
		c.queue.Forget(item)
		return true
	}

	// we get the config map from the lister
	configMap, err := c.lister.ConfigMaps(ns).Get(name)

	if err != nil {
		fmt.Println("Error while getting config map using lister ", err)
		return false
	}

	namespaces, err := c.clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})

	if err != nil {
		fmt.Println("Error while getting namespaces")
		return false
	}

	// TODO: check if configmap with same name already exists, and if yes compare the data
	for _, namespace := range namespaces.Items {
		if !contains(ignoredNamespaces, namespace.Name) {

			cm, err := c.clientset.CoreV1().ConfigMaps(namespace.Name).Get(context.Background(), configMap.Name, metav1.GetOptions{})

			if errors.IsNotFound(err) {
				cm, err = c.clientset.CoreV1().ConfigMaps(namespace.Name).Create(context.Background(), createConfigMap(namespace.Name, name, configMap.Data), metav1.CreateOptions{})
			}

			if err != nil {
				fmt.Println("Error while creating/getting config map: ", err)
				return false
			}

			if reflect.DeepEqual(cm.Data, configMap.Data) {
				_, err = c.clientset.CoreV1().ConfigMaps(namespace.Name).Update(context.Background(), createConfigMap(namespace.Name, name, configMap.Data), metav1.UpdateOptions{})
			}
		}
	}

	c.queue.Forget(item)
	return true
}

func createConfigMap(ns, name string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Data: data,
	}
}

func (c *controller) handleAdd(obj interface{}) {
	fmt.Println("Config map added")
	key, err := cache.MetaNamespaceKeyFunc(obj)

	if err != nil {
		fmt.Println("Error while creating key for object")
		return
	}

	c.queue.Add(key)
}

func (c *controller) handleDelete(obj interface{}) {
	fmt.Println("Config map deleted")
	key, err := cache.MetaNamespaceKeyFunc(obj)

	if err != nil {
		fmt.Println("Error while creating key for object")
		return
	}

	c.queue.Add(key)
}
