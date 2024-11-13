package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

func main() {
	// Use in-cluster configuration for Kubernetes client.
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(fmt.Sprintf("Failed to create in-cluster config: %v", err))
	}

	// Create the Kubernetes clientset.
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(fmt.Sprintf("Failed to create clientset: %v", err))
	}

	// Create a `SharedInformerFactory` for nodes with a resync period of 30 seconds.
	factory := informers.NewSharedInformerFactory(clientset, 30*time.Second)
	nodeInformer := factory.Core().V1().Nodes().Informer()

	// Add an event handler to handle node additions and updates.
	nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			node := obj.(*v1.Node)
			handleNode(clientset, node)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			node := newObj.(*v1.Node)
			handleNode(clientset, node)
		},
	})

	// Start the informer.
	stopCh := make(chan struct{})
	defer close(stopCh)
	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)

	// Wait indefinitely.
	select {}
}

func handleNode(clientset *kubernetes.Clientset, node *v1.Node) {
	partialNodeName := "system"
	// Check if the node name contains "system".
	if strings.Contains(node.Name, partialNodeName) {
		fmt.Printf("Node %s contains '%s' in its name. Applying taint...\n", node.Name, partialNodeName)

		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			// Get the current version of the node.
			currentNode, getErr := clientset.CoreV1().Nodes().Get(context.TODO(), node.Name, metav1.GetOptions{})
			if getErr != nil {
				return getErr
			}

			// Check if the taint is already present.
			hasTaint := false
			for _, taint := range currentNode.Spec.Taints {
				fmt.Printf("Taint: %s\n", taint.Key)
				if taint.Key == "CriticalAddonsOnly" && taint.Effect == v1.TaintEffectNoSchedule {
					hasTaint = true
					break
				}
			}

			if hasTaint {
				return nil // Skip if the taint is already present.
			}

			// Create the taint in the correct format.
			newTaint := v1.Taint{
				Key:    "CriticalAddonsOnly",
				Value:  "true",
				Effect: v1.TaintEffectNoSchedule,
			}
			currentNode.Spec.Taints = append(currentNode.Spec.Taints, newTaint)

			// Update the node with the new taint.
			_, updateErr := clientset.CoreV1().Nodes().Update(context.TODO(), currentNode, metav1.UpdateOptions{})
			return updateErr
		})
		if retryErr != nil {
			fmt.Printf("Failed to update node %s: %v\n", node.Name, retryErr)
		} else {
			fmt.Printf("Taint applied successfully to node %s\n", node.Name)
		}
	}
}
