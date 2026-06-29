package utils

import (
	"testing"

	"github.com/truefoundry/cruisekube/pkg/types"
)

func TestComputeCoordinatedHPATarget(t *testing.T) {
	tests := []struct {
		name       string
		targetOld  int32
		requestOld float64
		requestNew float64
		want       int32
		wantOK     bool
	}{
		{
			// Request halved -> target doubles to keep setpoint (0.3) constant.
			name: "request halved doubles target", targetOld: 30, requestOld: 1.0, requestNew: 0.5, want: 60, wantOK: true,
		},
		{
			// Request unchanged -> target unchanged.
			name: "request unchanged", targetOld: 60, requestOld: 1.0, requestNew: 1.0, want: 60, wantOK: true,
		},
		{
			// Request doubled -> target halves.
			name: "request doubled halves target", targetOld: 80, requestOld: 0.5, requestNew: 1.0, want: 40, wantOK: true,
		},
		{
			// Grossly over-requested: setpoint/requestNew exceeds MAX -> clamped.
			name: "clamped at max", targetOld: 70, requestOld: 1.0, requestNew: 0.1, want: HPAMaxTargetUtilization, wantOK: true,
		},
		{
			// requestNew far larger than setpoint -> below MIN -> clamped.
			name: "clamped at min", targetOld: 1, requestOld: 0.01, requestNew: 100, want: HPAMinTargetUtilization, wantOK: true,
		},
		{name: "zero target not ok", targetOld: 0, requestOld: 1, requestNew: 1, want: 0, wantOK: false},
		{name: "zero requestNew not ok", targetOld: 70, requestOld: 1, requestNew: 0, want: 0, wantOK: false},
		{name: "zero requestOld not ok", targetOld: 70, requestOld: 0, requestNew: 1, want: 0, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ComputeCoordinatedHPATarget(tt.targetOld, tt.requestOld, tt.requestNew)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Fatalf("target = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestComputeCoordinatedHPATargetPreservesSetpoint(t *testing.T) {
	// When not clamped, targetNew*requestNew should be within rounding of the
	// original setpoint targetOld*requestOld.
	targetOld := int32(40)
	requestOld := 0.8
	requestNew := 0.5
	got, ok := ComputeCoordinatedHPATarget(targetOld, requestOld, requestNew)
	if !ok {
		t.Fatal("expected ok")
	}
	setpointOld := float64(targetOld) * requestOld
	setpointNew := float64(got) * requestNew
	if diff := setpointOld - setpointNew; diff > requestNew || diff < -requestNew {
		t.Fatalf("setpoint drifted too far: old=%.3f new=%.3f", setpointOld, setpointNew)
	}
}

func TestCanonicalCPURequest(t *testing.T) {
	cs := &types.ContainerStats{
		CPUStats:             &types.CPUStats{P75: 0.3},
		SimplePredictionsCPU: &types.SimplePrediction{MaxValue: 0.5},
	}
	if got := CanonicalCPURequest(cs); got != 0.5 {
		t.Fatalf("CanonicalCPURequest = %v, want 0.5", got)
	}
	if got := CanonicalCPURequest(nil); got != 0 {
		t.Fatalf("CanonicalCPURequest(nil) = %v, want 0", got)
	}
}

func TestCanonicalMemoryRequest(t *testing.T) {
	cs := &types.ContainerStats{
		MemoryStats:             &types.MemoryStats{P75: 150, OOMMemory: 0},
		SimplePredictionsMemory: &types.SimplePrediction{MaxValue: 200},
	}
	if got := CanonicalMemoryRequest(cs); got != 200 {
		t.Fatalf("CanonicalMemoryRequest = %v, want 200", got)
	}
	// OOM observed memory dominates when higher.
	cs.MemoryStats.OOMMemory = 260
	if got := CanonicalMemoryRequest(cs); got != 260 {
		t.Fatalf("CanonicalMemoryRequest with OOM = %v, want 260", got)
	}
}
