package main

import (
	"encoding/json"
	"os"
	"testing"

	v1 "k8s.io/api/admission/v1"
)

func TestMutateIngress(t *testing.T) {
	data, err := os.ReadFile("admissionreview_sample.json")
	if err != nil {
		t.Fatalf("Failed to read sample data: %v", err)
	}

	var review v1.AdmissionReview
	err = json.Unmarshal(data, &review)
	if err != nil {
		t.Fatalf("Failed to unmarshal sample data: %v", err)
	}

	response := mutateIngress(review)
	if response.Result != nil {
		t.Fatal("Expected result to be nil")
	}

	// Additional assertions can be added here, for example, checking the patch.
}
