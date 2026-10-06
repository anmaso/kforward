package discovery

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type ResourceKind string

const (
	KindService    ResourceKind = "service"
	KindDeployment ResourceKind = "deployment"
)

type Target struct {
	Namespace string
	Name      string
	Kind      ResourceKind
	Ports     []int32
}

func (t Target) KindLabel() string {
	if t.Kind == KindDeployment {
		return "deploy"
	}
	return "svc"
}

func (t Target) PortsLabel() string {
	parts := make([]string, len(t.Ports))
	for i, p := range t.Ports {
		parts[i] = fmt.Sprintf("%d", p)
	}
	return strings.Join(parts, ",")
}

func (t Target) Label() string {
	if len(t.Ports) == 0 {
		return fmt.Sprintf("%s/%s (%s)", t.Namespace, t.Name, t.KindLabel())
	}
	return fmt.Sprintf("%s/%s (%s) ports [%s]", t.Namespace, t.Name, t.KindLabel(), strings.ReplaceAll(t.PortsLabel(), ",", ", "))
}

type Client struct {
	clientset *kubernetes.Clientset
	namespace string
}

func NewClient() (*Client, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}

	namespace, _, err := kubeConfig.Namespace()
	if err != nil {
		return nil, fmt.Errorf("resolve current namespace: %w", err)
	}
	if namespace == "" {
		namespace = "default"
	}

	return &Client{clientset: clientset, namespace: namespace}, nil
}

func (c *Client) CurrentNamespace() string {
	return c.namespace
}

func (c *Client) ListAllTargets(ctx context.Context) ([]Target, error) {
	svcs, err := c.clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	deps, err := c.clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}
	targets := matchServices(svcs.Items, "")
	targets = append(targets, matchDeployments(deps.Items, "")...)
	return targets, nil
}

func (c *Client) FindTargets(ctx context.Context, query string) ([]Target, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query must not be empty")
	}

	if targets, err := c.findServicesInNamespace(ctx, c.namespace, query); err != nil {
		return nil, err
	} else if len(targets) > 0 {
		return targets, nil
	}

	if targets, err := c.findDeploymentsInNamespace(ctx, c.namespace, query); err != nil {
		return nil, err
	} else if len(targets) > 0 {
		return targets, nil
	}

	if targets, err := c.findServicesAllNamespaces(ctx, query); err != nil {
		return nil, err
	} else if len(targets) > 0 {
		return targets, nil
	}

	return c.findDeploymentsAllNamespaces(ctx, query)
}

func (c *Client) ResolveTarget(ctx context.Context, namespace, name string) (Target, error) {
	svc, err := c.clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return Target{
			Namespace: namespace,
			Name:      name,
			Kind:      KindService,
			Ports:     servicePorts(*svc),
		}, nil
	}

	dep, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return Target{
			Namespace: namespace,
			Name:      name,
			Kind:      KindDeployment,
			Ports:     deploymentPorts(*dep),
		}, nil
	}

	return Target{}, fmt.Errorf("service or deployment %s/%s not found", namespace, name)
}

func (c *Client) findServicesInNamespace(ctx context.Context, namespace, query string) ([]Target, error) {
	list, err := c.clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list services in namespace %q: %w", namespace, err)
	}
	return matchServices(list.Items, query), nil
}

func (c *Client) findDeploymentsInNamespace(ctx context.Context, namespace, query string) ([]Target, error) {
	list, err := c.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list deployments in namespace %q: %w", namespace, err)
	}
	return matchDeployments(list.Items, query), nil
}

func (c *Client) findServicesAllNamespaces(ctx context.Context, query string) ([]Target, error) {
	list, err := c.clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list services in all namespaces: %w", err)
	}
	return matchServices(list.Items, query), nil
}

func (c *Client) findDeploymentsAllNamespaces(ctx context.Context, query string) ([]Target, error) {
	list, err := c.clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list deployments in all namespaces: %w", err)
	}
	return matchDeployments(list.Items, query), nil
}

func matchServices(services []corev1.Service, query string) []Target {
	var targets []Target
	for _, svc := range services {
		if !partialMatch(svc.Name, query) {
			continue
		}
		ports := servicePorts(svc)
		if len(ports) == 0 {
			continue
		}
		targets = append(targets, Target{
			Namespace: svc.Namespace,
			Name:      svc.Name,
			Kind:      KindService,
			Ports:     ports,
		})
	}
	return targets
}

func matchDeployments(deployments []appsv1.Deployment, query string) []Target {
	var targets []Target
	for _, dep := range deployments {
		if !partialMatch(dep.Name, query) {
			continue
		}
		ports := deploymentPorts(dep)
		if len(ports) == 0 {
			continue
		}
		targets = append(targets, Target{
			Namespace: dep.Namespace,
			Name:      dep.Name,
			Kind:      KindDeployment,
			Ports:     ports,
		})
	}
	return targets
}

func partialMatch(name, query string) bool {
	return strings.Contains(strings.ToLower(name), strings.ToLower(query))
}

func servicePorts(svc corev1.Service) []int32 {
	ports := make([]int32, 0, len(svc.Spec.Ports))
	for _, p := range svc.Spec.Ports {
		port := p.Port
		if p.TargetPort.Type == 0 && p.TargetPort.IntVal > 0 {
			port = p.TargetPort.IntVal
		}
		ports = append(ports, port)
	}
	return ports
}

func deploymentPorts(dep appsv1.Deployment) []int32 {
	if dep.Spec.Template.Spec.Containers == nil {
		return nil
	}
	seen := make(map[int32]struct{})
	var ports []int32
	for _, container := range dep.Spec.Template.Spec.Containers {
		for _, p := range container.Ports {
			if _, ok := seen[p.ContainerPort]; ok {
				continue
			}
			seen[p.ContainerPort] = struct{}{}
			ports = append(ports, p.ContainerPort)
		}
	}
	return ports
}
