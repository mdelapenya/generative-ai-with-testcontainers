package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// GPUMetrics holds GPU utilization and memory usage
type GPUMetrics struct {
	Utilization float64 // GPU utilization percentage
	MemoryUsed  float64 // GPU memory used in MB
	Available   bool    // Whether GPU metrics are available
}

// GPUSampler is an interface for sampling GPU metrics from different vendors
type GPUSampler interface {
	Sample() (*GPUMetrics, error)
	IsAvailable() bool
}

// GPUVendor represents the detected GPU vendor
type GPUVendor int

const (
	GPUVendorUnknown GPUVendor = iota
	GPUVendorNVIDIA
	GPUVendorApple
)

// DetectGPUVendor detects which GPU vendor is present in the system
func DetectGPUVendor() GPUVendor {
	// Check for Apple Silicon (macOS on ARM)
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		return GPUVendorApple
	}

	// Check for NVIDIA GPU
	cmd := exec.Command("nvidia-smi", "-L")
	if err := cmd.Run(); err == nil {
		return GPUVendorNVIDIA
	}

	return GPUVendorUnknown
}

// NewGPUSampler creates a GPU sampler based on the detected vendor
func NewGPUSampler() GPUSampler {
	vendor := DetectGPUVendor()

	switch vendor {
	case GPUVendorNVIDIA:
		return &NVIDIAGPUSampler{}
	case GPUVendorApple:
		return &AppleGPUSampler{}
	default:
		return &NoOpGPUSampler{}
	}
}

// NVIDIAGPUSampler samples GPU metrics from NVIDIA GPUs using nvidia-smi
type NVIDIAGPUSampler struct{}

func (s *NVIDIAGPUSampler) IsAvailable() bool {
	cmd := exec.Command("nvidia-smi", "-L")
	return cmd.Run() == nil
}

func (s *NVIDIAGPUSampler) Sample() (*GPUMetrics, error) {
	cmd := exec.Command("nvidia-smi",
		"--query-gpu=utilization.gpu,memory.used",
		"--format=csv,noheader,nounits")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return &GPUMetrics{
			Utilization: 0,
			MemoryUsed:  0,
			Available:   false,
		}, nil
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return &GPUMetrics{
			Utilization: 0,
			MemoryUsed:  0,
			Available:   false,
		}, nil
	}

	// Parse output: "utilization, memory_used"
	parts := strings.Split(output, ",")
	if len(parts) < 2 {
		return nil, fmt.Errorf("unexpected nvidia-smi output format: %s", output)
	}

	utilization, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse utilization: %w", err)
	}

	memoryUsed, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse memory used: %w", err)
	}

	return &GPUMetrics{
		Utilization: utilization,
		MemoryUsed:  memoryUsed,
		Available:   true,
	}, nil
}

// AppleGPUSampler samples GPU metrics from Apple Silicon GPUs
// Uses ioreg and other non-privileged commands to avoid requiring sudo
type AppleGPUSampler struct{}

func (s *AppleGPUSampler) IsAvailable() bool {
	// Check if we're on macOS ARM
	return runtime.GOOS == "darwin" && runtime.GOARCH == "arm64"
}

func (s *AppleGPUSampler) Sample() (*GPUMetrics, error) {
	// Approach: Use ioreg to extract GPU metrics without sudo
	// This provides memory usage and some performance statistics

	memoryUsed := s.getGPUMemory()
	utilization := s.getGPUUtilization()

	// Consider metrics available if we got at least memory info
	available := memoryUsed > 0 || utilization > 0

	return &GPUMetrics{
		Utilization: utilization,
		MemoryUsed:  memoryUsed,
		Available:   available,
	}, nil
}

func (s *AppleGPUSampler) getGPUUtilization() float64 {
	// Get GPU activity from ioreg PerformanceStatistics
	// Apple Silicon GPUs expose utilization metrics through ioreg
	cmd := exec.Command("ioreg", "-r", "-c", "IOAccelerator", "-d", "2")

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return 0
	}

	output := stdout.String()

	// Look for "Device Utilization %" in PerformanceStatistics
	// Format: "Device Utilization %"=<number> (note: no space after =)
	// Try multiple utilization metrics in order of preference

	// Method 1: Device Utilization (overall GPU usage)
	deviceUtil := s.extractMetric(output, `"Device Utilization %"\s*=\s*(\d+(?:\.\d+)?)`)
	if deviceUtil > 0 {
		return deviceUtil
	}

	// Method 2: Renderer Utilization (rendering workload)
	rendererUtil := s.extractMetric(output, `"Renderer Utilization %"\s*=\s*(\d+(?:\.\d+)?)`)
	if rendererUtil > 0 {
		return rendererUtil
	}

	// Method 3: Tiler Utilization (geometry processing)
	tilerUtil := s.extractMetric(output, `"Tiler Utilization %"\s*=\s*(\d+(?:\.\d+)?)`)
	if tilerUtil > 0 {
		return tilerUtil
	}

	// Method 4: Try to calculate from active/idle ticks if available
	activeTicks := s.extractMetric(output, `"GPU Active Ticks"\s*=\s*(\d+)`)
	idleTicks := s.extractMetric(output, `"GPU Idle Ticks"\s*=\s*(\d+)`)

	if activeTicks > 0 && (activeTicks+idleTicks) > 0 {
		totalTicks := activeTicks + idleTicks
		return (activeTicks / totalTicks) * 100.0
	}

	// No utilization metrics available (GPU likely idle or metrics not exposed)
	return 0
}

func (s *AppleGPUSampler) extractMetric(text, pattern string) float64 {
	re := regexp.MustCompile(pattern)
	if matches := re.FindStringSubmatch(text); len(matches) > 1 {
		value, err := strconv.ParseFloat(matches[1], 64)
		if err == nil {
			return value
		}
	}
	return 0
}

func (s *AppleGPUSampler) getGPUMemory() float64 {
	// Use ioreg to get GPU memory allocation info
	cmd := exec.Command("ioreg", "-r", "-c", "IOAccelerator", "-d", "2")

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return 0
	}

	output := stdout.String()

	// Method 1: Look for "Alloc system memory" in PerformanceStatistics
	// This shows the total allocated system memory for the GPU
	// Format: "Alloc system memory"=<bytes>
	allocMem := s.extractMetric(output, `"Alloc system memory"\s*=\s*(\d+)`)
	if allocMem > 0 {
		return allocMem / (1024 * 1024) // Convert bytes to MB
	}

	// Method 2: Look for "In use system memory (driver)"
	// This shows memory actively used by the driver
	inUseMem := s.extractMetric(output, `"In use system memory \(driver\)"\s*=\s*(\d+)`)
	if inUseMem > 0 {
		return inUseMem / (1024 * 1024) // Convert bytes to MB
	}

	// Method 3: Look for IOAcceleratorAllocatedMemory (older format)
	allocMemLegacy := s.extractMetric(output, `"IOAcceleratorAllocatedMemory"\s*=\s*(\d+)`)
	if allocMemLegacy > 0 {
		return allocMemLegacy / (1024 * 1024) // Convert bytes to MB
	}

	// Method 4: Try PerformanceStatistics vram usage
	vramUsed := s.extractMetric(output, `"vramUsedBytes"\s*=\s*(\d+)`)
	if vramUsed > 0 {
		return vramUsed / (1024 * 1024) // Convert bytes to MB
	}

	return 0
}

// NoOpGPUSampler returns zero metrics when no GPU is detected
type NoOpGPUSampler struct{}

func (s *NoOpGPUSampler) IsAvailable() bool {
	return false
}

func (s *NoOpGPUSampler) Sample() (*GPUMetrics, error) {
	return &GPUMetrics{
		Utilization: 0,
		MemoryUsed:  0,
		Available:   false,
	}, nil
}

// SampleGPU samples GPU metrics using the appropriate sampler for the system
// This is a convenience function that maintains backward compatibility
func SampleGPU() (*GPUMetrics, error) {
	sampler := NewGPUSampler()
	return sampler.Sample()
}
