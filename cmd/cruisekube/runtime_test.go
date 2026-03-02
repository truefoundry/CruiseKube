package main

import (
	"testing"

	"github.com/truefoundry/cruisekube/pkg/config"
)

func TestShouldStartWebhook(t *testing.T) {
	testCases := []struct {
		name     string
		mode     config.ExecutionMode
		expected bool
	}{
		{name: "webhook", mode: config.ExecutionModeWebhook, expected: true},
		{name: "both", mode: config.ExecutionModeBoth, expected: true},
		{name: "controller", mode: config.ExecutionModeController, expected: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if actual := shouldStartWebhook(testCase.mode); actual != testCase.expected {
				t.Fatalf("shouldStartWebhook(%q) = %t, want %t", testCase.mode, actual, testCase.expected)
			}
		})
	}
}

func TestShouldStartController(t *testing.T) {
	testCases := []struct {
		name     string
		mode     config.ExecutionMode
		expected bool
	}{
		{name: "controller", mode: config.ExecutionModeController, expected: true},
		{name: "both", mode: config.ExecutionModeBoth, expected: true},
		{name: "webhook", mode: config.ExecutionModeWebhook, expected: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if actual := shouldStartController(testCase.mode); actual != testCase.expected {
				t.Fatalf("shouldStartController(%q) = %t, want %t", testCase.mode, actual, testCase.expected)
			}
		})
	}
}

func TestShouldBlockForever(t *testing.T) {
	testCases := []struct {
		name     string
		mode     config.ExecutionMode
		expected bool
	}{
		{name: "webhook", mode: config.ExecutionModeWebhook, expected: true},
		{name: "both", mode: config.ExecutionModeBoth, expected: false},
		{name: "controller", mode: config.ExecutionModeController, expected: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if actual := shouldBlockForever(testCase.mode); actual != testCase.expected {
				t.Fatalf("shouldBlockForever(%q) = %t, want %t", testCase.mode, actual, testCase.expected)
			}
		})
	}
}
