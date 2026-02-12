package main

import (
	"sync"
)

// GPUDeltaSampler tracks baseline GPU memory and calculates deltas
type GPUDeltaSampler struct {
	sampler      GPUSampler
	baselineUtil float64
	baselineMem  float64
	mu           sync.RWMutex
	hasBaseline  bool
}

// NewGPUDeltaSampler creates a new delta sampler with baseline tracking
func NewGPUDeltaSampler() *GPUDeltaSampler {
	return &GPUDeltaSampler{
		sampler: NewGPUSampler(),
	}
}

// CaptureBaseline captures the current GPU state as baseline
// Call this before loading models to get accurate delta measurements
func (s *GPUDeltaSampler) CaptureBaseline() error {
	metrics, err := s.sampler.Sample()
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if metrics.Available {
		s.baselineUtil = metrics.Utilization
		s.baselineMem = metrics.MemoryUsed
		s.hasBaseline = true
	}

	return nil
}

// SampleDelta returns GPU metrics as delta from baseline
// If no baseline is captured, returns absolute values
func (s *GPUDeltaSampler) SampleDelta() (*GPUMetrics, error) {
	current, err := s.sampler.Sample()
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.hasBaseline || !current.Available {
		// No baseline or metrics unavailable - return as-is
		return current, nil
	}

	// Calculate delta from baseline
	deltaUtil := current.Utilization - s.baselineUtil
	deltaMem := current.MemoryUsed - s.baselineMem

	// Ensure non-negative values (GPU memory can decrease if other processes free memory)
	if deltaUtil < 0 {
		deltaUtil = 0
	}
	if deltaMem < 0 {
		deltaMem = 0
	}

	return &GPUMetrics{
		Utilization: deltaUtil,
		MemoryUsed:  deltaMem,
		Available:   true,
	}, nil
}

// IsAvailable returns whether GPU metrics are available
func (s *GPUDeltaSampler) IsAvailable() bool {
	return s.sampler.IsAvailable()
}

// HasBaseline returns whether a baseline has been captured
func (s *GPUDeltaSampler) HasBaseline() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hasBaseline
}
