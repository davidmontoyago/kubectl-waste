package cmd

import (
	"k8s.io/apimachinery/pkg/api/resource"
	"testing"
)

func NewTestPod() Pod {
  pod := Pod{Name: "test-pod",
    RequestedMem: resource.MustParse("1"),
    RequestedCpu: resource.MustParse("1000m"),
  }
  container1 := Container{Name: "test-container-1",
    UsedMem: resource.MustParse("100m"),
    UsedCpu: resource.MustParse("10m"),
  }
  container2 := Container{Name: "test-container-2",
    UsedMem: resource.MustParse("100m"),
    UsedCpu: resource.MustParse("10m"),
  }
  pod.Containers = []Container{container1, container2}
  return pod
}

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
