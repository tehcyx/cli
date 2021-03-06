package kube

import (
	"fmt"
	"strings"
	"time"

	"github.com/kyma-project/cli/pkg/api/octopus"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	istioNet "github.com/kyma-project/kyma/components/api-controller/pkg/clients/networking.istio.io/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultHTTPTimeout = 30 * time.Second
	defaultWaitSleep   = 3 * time.Second
)

// client is the default KymaKube implementation
type client struct {
	static  kubernetes.Interface
	dynamic dynamic.Interface
	octps   octopus.Interface
	istio   istioNet.Interface
	cfg     *rest.Config
}

// NewFromConfig creates a new Kubernetes client based on the given Kubeconfig either provided by URL (in-cluster config) or via file (out-of-cluster config).
func NewFromConfig(url, file string) (KymaKube, error) {
	return NewFromConfigWithTimeout(url, file, defaultHTTPTimeout)
}

// NewFromConfigWithTimeout creates a new Kubernetes client based on the given Kubeconfig either provided by URL (in-cluster config) or via file (out-of-cluster config).
// Allows to set a custom timeout for the Kubernetes HTTP client.
func NewFromConfigWithTimeout(url, file string, t time.Duration) (KymaKube, error) {
	config, err := Kubeconfig(url, file)
	if err != nil {
		return nil, err
	}

	config.Timeout = t

	sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	octClient, err := octopus.NewFromConfig(config)
	if err != nil {
		return nil, err
	}

	istioClient, err := istioNet.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &client{
			static:  sClient,
			dynamic: dClient,
			octps:   octClient,
			istio:   istioClient,
			cfg:     config,
		},
		nil

}

func (c *client) Static() kubernetes.Interface {
	return c.static
}

func (c *client) Dynamic() dynamic.Interface {
	return c.dynamic
}

func (c *client) Octopus() octopus.Interface {
	return c.octps
}

func (c *client) Istio() istioNet.Interface {
	return c.istio
}

func (c *client) Config() *rest.Config {
	return c.cfg
}

func (c *client) IsPodDeployed(namespace, name string) (bool, error) {
	_, err := c.Static().CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		// actual errors
		return false, err
	}
	return true, nil
}

func (c *client) IsPodDeployedByLabel(namespace, labelName, labelValue string) (bool, error) {
	pods, err := c.Static().CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", labelName, labelValue)})
	if err != nil {
		return false, err
	}

	return len(pods.Items) > 0, nil
}

func (c *client) WaitPodStatus(namespace, name string, status corev1.PodPhase) error {
	for {
		pod, err := c.Static().CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
		if err != nil && !strings.Contains(err.Error(), "not found") {
			return err
		}

		if status == pod.Status.Phase {
			return nil
		}
		time.Sleep(defaultWaitSleep)
	}
}

func (c *client) WaitPodStatusByLabel(namespace, labelName, labelValue string, status corev1.PodPhase) error {
	for {
		pods, err := c.Static().CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", labelName, labelValue)})
		if err != nil {
			return err
		}

		ok := true
		for _, pod := range pods.Items {
			// if any pod is not in the desired status no need to check further
			if status != pod.Status.Phase {
				ok = false
				break
			}
		}
		if ok {
			return nil
		}
		time.Sleep(defaultWaitSleep)
	}
}
