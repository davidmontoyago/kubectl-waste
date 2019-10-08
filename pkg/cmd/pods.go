package cmd

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"k8s.io/apimachinery/pkg/api/resource"
	metrics "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
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

func (container Container) MemUtilizationPercentage() float64 {
	used := container.UsedMem.MilliValue()
	requested := container.RequestedMem.MilliValue()
	return float64(used) / float64(requested) * 100
}

func (container Container) CpuUtilizationPercentage() float64 {
	used := container.UsedCpu.MilliValue()
	requested := container.RequestedCpu.MilliValue()
	return float64(used) / float64(requested) * 100
}

type Pod struct {
	Name       string
	Namespace  string
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

// Return True if at least one container is Cpu bound
func (pod Pod) IsCpuBound() bool {
	for _, container := range pod.Containers {
		if container.IsCpuBound() {
			return true
		}
	}
	return false
}

// Return True if at least one container is Mem bound
func (pod Pod) IsMemBound() bool {
	for _, container := range pod.Containers {
		if container.IsMemBound() {
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

func (this Pod) HasLessCpuUtilizationThan(another Pod) bool {
	if this.IsCpuBound() && another.IsCpuBound() {
		return this.CpuUtilizationPercentage() < another.CpuUtilizationPercentage()
	} else if this.IsCpuBound() {
		return true
	}
	return false
}

func (this Pod) HasLessMemUtilizationThan(another Pod) bool {
	if this.IsMemBound() && another.IsMemBound() {
		return this.MemUtilizationPercentage() < another.MemUtilizationPercentage()
	} else if this.IsMemBound() {
		return true
	}
	return false
}

func (pod Pod) HasLowUtilization(utilizationThreshold float64) bool {
	if pod.IsMemBound() && pod.MemUtilizationPercentage() < utilizationThreshold {
		return true
	} else if pod.IsCpuBound() && pod.CpuUtilizationPercentage() < utilizationThreshold {
		return true
	}
	return false
}

type ByUtilization []Pod

func (pods ByUtilization) Len() int      { return len(pods) }
func (pods ByUtilization) Swap(i, j int) { pods[i], pods[j] = pods[j], pods[i] }
func (pods ByUtilization) Less(i, j int) bool {
	if pods[i].HasLessCpuUtilizationThan(pods[j]) {
		return true
	} else if pods[i].HasLessMemUtilizationThan(pods[j]) {
		return true
	}
	return false
}

func findPods(namespace string,
	corev1Client typev1.CoreV1Interface,
	metricsv1Client metricsv1beta1.MetricsV1beta1Interface) ([]Pod, error) {

	listOptions := metav1.ListOptions{}
	allPods, err := corev1Client.Pods(namespace).List(listOptions)
	if err != nil {
		return nil, err
	}
	podsMetrics, err := metricsv1Client.PodMetricses(namespace).List(listOptions)
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
	pods = Filter(pods, func(pod Pod) bool {
		utilizationThresholdPercent := 50.0
		return pod.HasLowUtilization(utilizationThresholdPercent)
	})

	sort.Sort(ByUtilization(pods))

	return pods, nil
}

func collectPodsRequests(allPods *corev1.PodList) (map[string]Pod, error) {
	var podsByName = make(map[string]Pod)
	for _, pod := range allPods.Items {
		podContainers := make(map[string]Container)
		consumingPod := Pod{Name: pod.Name, Namespace: pod.Namespace}
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
	fmt.Fprintln(w, "NAMESPACE\tNAME\tMEM REQUESTED\tMEM UTILIZATION %\tCPU REQUESTED\tCPU UTILIZATION %\t")
	fmt.Fprintln(w, "---------\t----\t-------------\t-----------------\t-------------\t-----------------\t")
	for _, pod := range pods {
		fmt.Fprintln(w, toPodRow(pod))

		for _, container := range pod.Containers {
			fmt.Fprintln(w, toContainerRow(container))
		}
	}

	w.Flush()
}

func toPodRow(pod Pod) string {
	totalRequestedMemFormatted := "-"
	memUtilizationPercentFormatted := "-"
	if pod.IsMemBound() {
		totalRequestedMem := pod.TotalRequestedMem()
		totalRequestedMemFormatted = totalRequestedMem.String()
		memUtilizationPercentFormatted = fmt.Sprintf("%2.f%%", pod.MemUtilizationPercentage())
	}

	totalRequestedCpuFormatted := "-"
	cpuUtilizationPercetFormatted := "-"
	if pod.IsCpuBound() {
		totalRequestedCpu := pod.TotalRequestedCpu()
		totalRequestedCpuFormatted = totalRequestedCpu.String()
		cpuUtilizationPercetFormatted = fmt.Sprintf("%2.f%%", pod.CpuUtilizationPercentage())
	}

	podRow := fmt.Sprintf("%s\t%s\t%v\t%s\t%v\t%s\t",
		pod.Namespace, pod.Name,
		totalRequestedMemFormatted, memUtilizationPercentFormatted,
		totalRequestedCpuFormatted, cpuUtilizationPercetFormatted)

	return podRow
}

func toContainerRow(container Container) string {
	memUtilizationFormatted := "-"
	if container.IsMemBound() {
		memUtilizationFormatted = fmt.Sprintf("%2.f%%", container.MemUtilizationPercentage())
	}

	cpuUtilizationFormatted := "-"
	if container.IsCpuBound() {
		cpuUtilizationFormatted = fmt.Sprintf("%2.f%%", container.CpuUtilizationPercentage())
	}

	containerRow := fmt.Sprintf("%s\t%s\t%v\t%s\t%v\t%s\t",
		"", "  \\_"+container.Name,
		container.RequestedMem.String(), memUtilizationFormatted,
		container.RequestedCpu.String(), cpuUtilizationFormatted)

	return containerRow
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
