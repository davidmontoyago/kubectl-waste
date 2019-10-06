package cmd

import (
  "fmt"
	"os"
  "text/tabwriter"

  corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	metrics "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
  "k8s.io/apimachinery/pkg/api/resource"
)

type Container struct {
	Name         string
	UsedMem      resource.Quantity
	UsedCpu      resource.Quantity
	RequestedMem resource.Quantity
	RequestedCpu resource.Quantity
}

func (container Container) IsCpuBound() bool {
	return container.RequestedCpu.Value() != 0
}

func (container Container) IsMemBound() bool {
	return container.RequestedMem.Value() != 0
}

type Pod struct {
	Name       string
	Containers map[string]Container
}

func (pod Pod) MemUtilizationPercentage() float64 {
	var totalUsedMem, totalRequestedMem int64
	for _, container := range pod.Containers {
		if container.IsMemBound() {
			totalUsedMem += container.UsedMem.MilliValue()
			totalRequestedMem += container.RequestedMem.MilliValue()
		}
	}
	return float64(totalUsedMem) / float64(totalRequestedMem) * 100
}

func (pod Pod) CpuUtilizationPercentage() float64 {
	var totalUsedCpu, totalRequestedCpu int64
	for _, container := range pod.Containers {
		if container.IsCpuBound() {
			totalUsedCpu += container.UsedCpu.MilliValue()
			totalRequestedCpu += container.RequestedCpu.MilliValue()
		}
	}
	return float64(totalUsedCpu) / float64(totalRequestedCpu) * 100
}

func (pod Pod) IsResourceBound() bool {
	for _, container := range pod.Containers {
		if container.IsMemBound() || container.IsCpuBound() {
			return true
		}
	}
	return false
}

func (pod Pod) TotalRequestedCpu() resource.Quantity {
	totalRequested := resource.NewMilliQuantity(0, resource.DecimalSI)
	for _, container := range pod.Containers {
		if container.IsCpuBound() {
			totalRequested.Add(container.RequestedCpu)
		}
	}
	return *totalRequested
}

func (pod Pod) TotalRequestedMem() resource.Quantity {
	totalRequested := resource.NewMilliQuantity(0, resource.DecimalSI)
	for _, container := range pod.Containers {
		if container.IsMemBound() {
			totalRequested.Add(container.RequestedMem)
		}
	}
	return *totalRequested
}

func findPods(namespace string,
	corev1Client typev1.CoreV1Interface,
	metricsv1Client metricsv1beta1.MetricsV1beta1Interface) ([]Pod, error) {

	listOptions := metav1.ListOptions{}
	allPods, err := corev1Client.Pods(metav1.NamespaceAll).List(listOptions)
	if err != nil {
		return nil, err
	}
	podsMetrics, err := metricsv1Client.PodMetricses(metav1.NamespaceAll).List(listOptions)
	if err != nil {
		return nil, err
	}

	podsByName, err := collectPodsRequests(allPods)
	if err != nil {
		return nil, err
	}

	pods, err := collectPodsMetrics(podsByName, podsMetrics)
	if err != nil {
		return nil, err
	}

	pods = Filter(pods, Pod.IsResourceBound)

	return pods, nil
}

func collectPodsRequests(allPods *corev1.PodList) (map[string]Pod, error) {
	var podsByName = make(map[string]Pod)
	for _, pod := range allPods.Items {
		podContainers := make(map[string]Container)
		consumingPod := Pod{Name: pod.Name}
		for _, container := range pod.Spec.Containers {
			resources := container.Resources
			podContainer := Container{Name: container.Name}
			podContainer.RequestedMem = resources.Requests[corev1.ResourceMemory]
			podContainer.RequestedCpu = resources.Requests[corev1.ResourceCPU]
			podContainers[container.Name] = podContainer
		}
		consumingPod.Containers = podContainers
		podsByName[pod.Name] = consumingPod
	}
	return podsByName, nil
}

func collectPodsMetrics(podsByName map[string]Pod,
	allPodsMetrics *metrics.PodMetricsList) ([]Pod, error) {

	foundPods := []Pod{}
	for _, podMetric := range allPodsMetrics.Items {
		consumingPod := podsByName[podMetric.ObjectMeta.Name]
		containersByName := consumingPod.Containers
		for _, container := range podMetric.Containers {
			podContainer := containersByName[container.Name]
			podContainer.UsedMem = *container.Usage.Memory()
			podContainer.UsedCpu = *container.Usage.Cpu()
			containersByName[container.Name] = podContainer
		}
		consumingPod.Containers = containersByName
		foundPods = append(foundPods, consumingPod)
	}
	return foundPods, nil
}

func printPods(pods []Pod) {
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 1, '\t', 0)
	fmt.Fprintln(w, "Name\tMem Requested\tMem Utilization %\tCpu Requested\tCpu Utilization %\t")
	fmt.Fprintln(w, "----\t-------------\t-----------------\t-------------\t-----------------\t")
	for _, pod := range pods {
		totalRequestedMem := pod.TotalRequestedMem()
		totalRequestedCpu := pod.TotalRequestedCpu()
		row := fmt.Sprintf("%s\t%v\t%2.f%%\t%v\t%2.f%%\t",
			pod.Name, totalRequestedMem.String(), pod.MemUtilizationPercentage(),
			totalRequestedCpu.String(), pod.CpuUtilizationPercentage())
		fmt.Fprintln(w, row)
	}

	w.Flush()
}

func Filter(vs []Pod, f func(Pod) bool) []Pod {
	vsf := make([]Pod, 0)
	for _, v := range vs {
		if f(v) {
			vsf = append(vsf, v)
		}
	}
	return vsf
}
