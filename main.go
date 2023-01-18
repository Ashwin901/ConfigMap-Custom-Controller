package main

import (
	"fmt"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	fmt.Println("When a config map is created, the controller will create the same config map across all namespaces")

	configFilePath := "/home/ashwin901/.kube/config"
	config, err := clientcmd.BuildConfigFromFlags("", configFilePath)

	if err != nil {
		fmt.Println("Error while building config", err.Error())
		return
	}

	clientset, err := kubernetes.NewForConfig(config)

	// Creating a new factory and setting the resync time to 10 minutes
	factory := informers.NewSharedInformerFactory(clientset, 10*time.Minute)

	ch := make(chan struct{}) // used as a stop channel

	controller := newController(clientset, factory.Core().V1().ConfigMaps())

	// initialising all the requested informers
	factory.Start(ch)
}
