package cmd

import (
	"k8s.io/apimachinery/pkg/api/resource"
	"testing"
)

// ---------------
// Pod tests
// ---------------
func TestMemUtilizationPercentage(t *testing.T) {
	pod := NewTestPod()
	utilizationPerct := pod.MemUtilizationPercentage()
	if utilizationPerct != 20.00 {
		t.Errorf("got = %.2f; want 20.00", utilizationPerct)
	}
}

func TestCpuUtilizationPercentage(t *testing.T) {
	pod := NewTestPod()
	utilizationPerct := pod.CpuUtilizationPercentage()
	if utilizationPerct != 2.00 {
		t.Errorf("got = %.2f; want 2.00", utilizationPerct)
	}
}

func TestIsResourceBoundPod(t *testing.T) {
	pod := NewTestPodWithNoRequests()
	isResourceBound := pod.IsResourceBound()
	if isResourceBound {
		t.Errorf("got = %v; want false", isResourceBound)
	}
}

func TestIsCpuBoundPod(t *testing.T) {
	pod := NewTestPodWithNoRequests()
	isResourceBound := pod.IsCpuBound()
	if isResourceBound {
		t.Errorf("got = %v; want false", isResourceBound)
	}
}

func TestIsMemBoundPod(t *testing.T) {
	pod := NewTestPodWithNoRequests()
	isResourceBound := pod.IsMemBound()
	if isResourceBound {
		t.Errorf("got = %v; want false", isResourceBound)
	}
}

func TestTotalRequestedCpu(t *testing.T) {
	pod := NewTestPod()
	totalRequested := pod.TotalRequestedCpu()
	if totalRequested.MilliValue() != 1000 {
		t.Errorf("got = %v; want 1000", totalRequested)
	}
}

func TestTotalRequestedMem(t *testing.T) {
	pod := NewTestPod()
	totalRequested := pod.TotalRequestedMem()
	if totalRequested.String() != "1" {
		t.Errorf("got = %v; want 1", totalRequested)
	}
}

// ---------------
// Container tests
// ---------------
func TestIsCpuBoundContainer(t *testing.T) {
	pod := NewTestPod()
	unboundContainer := pod.Containers["resource-unbound-container"]
	isCpuBound := unboundContainer.IsCpuBound()
	if isCpuBound {
		t.Errorf("got = %v; want false", isCpuBound)
	}
}

func TestIsMemBoundContainer(t *testing.T) {
	pod := NewTestPod()
	unboundContainer := pod.Containers["resource-unbound-container"]
	isMemBound := unboundContainer.IsMemBound()
	if isMemBound {
		t.Errorf("got = %v; want false", isMemBound)
	}
}

// ---------------
// Fixtures
// ---------------
func NewTestPod() Pod {
	pod := Pod{Name: "test-pod"}
	container1 := Container{Name: "test-container-1",
		UsedMem:      resource.MustParse("100m"),
		UsedCpu:      resource.MustParse("10m"),
		RequestedMem: resource.MustParse("0.5"),
		RequestedCpu: resource.MustParse("500m"),
	}
	container2 := Container{Name: "test-container-2",
		UsedMem:      resource.MustParse("100m"),
		UsedCpu:      resource.MustParse("10m"),
		RequestedMem: resource.MustParse("0.5"),
		RequestedCpu: resource.MustParse("500m"),
	}
	resourceUnboundContainer := Container{Name: "resource-unbound-container",
		UsedMem:      resource.MustParse("50m"),
		UsedCpu:      resource.MustParse("5m"),
		RequestedMem: resource.MustParse("0"),
		RequestedCpu: resource.MustParse("0"),
	}
	pod.Containers = map[string]Container{
		"test-container-1":           container1,
		"test-container-2":           container2,
		"resource-unbound-container": resourceUnboundContainer,
	}
	return pod
}

func NewTestPodWithNoRequests() Pod {
	pod := Pod{Name: "test-pod"}
	container1 := Container{Name: "resource-unbound-container-1",
		UsedMem:      resource.MustParse("100m"),
		UsedCpu:      resource.MustParse("10m"),
		RequestedMem: resource.MustParse("0"),
		RequestedCpu: resource.MustParse("0"),
	}
	container2 := Container{Name: "resource-unbound-container-2",
		UsedMem:      resource.MustParse("50m"),
		UsedCpu:      resource.MustParse("5m"),
		RequestedMem: resource.MustParse("0"),
		RequestedCpu: resource.MustParse("0"),
	}
	pod.Containers = map[string]Container{
		"resource-unbound-container-1": container1,
		"resource-unbound-container-2": container2,
	}
	return pod
}
