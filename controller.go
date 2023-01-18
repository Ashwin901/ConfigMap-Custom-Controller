package main

import (
	coreInformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	coreListers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

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

func (c *controller) handleAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)

	if err != nil {
		// log error
		return
	}

	c.queue.Add(key)

}

func (c *controller) handleDelete(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)

	if err != nil {
		// log error
		return
	}

	c.queue.Add(key)
}
