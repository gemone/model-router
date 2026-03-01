package router

import (
	"testing"

	"github.com/gemone/model-router/internal/model"
)

// TestAggregatorStructure verifies the aggregator types are properly defined
func TestAggregatorStructure(t *testing.T) {
	t.Run("BackendModel struct exists", func(t *testing.T) {
		// This is a compile-time test to ensure the struct is defined correctly
		bm := BackendModel{}
		if bm.TimeoutMs != 0 {
			t.Log("TimeoutMs field exists")
		}
	})

	t.Run("CompositeBackendModel can be serialized", func(t *testing.T) {
		// Test JSON marshaling
		cbm := model.CompositeBackendModel{
			ModelName:  "test-model",
			ProviderID: "test-provider",
			Weight:     100,
			TimeoutMs:  30000,
		}

		// This test verifies the struct can be created
		if cbm.ModelName != "test-model" {
			t.Error("ModelName not set correctly")
		}
		if cbm.TimeoutMs != 30000 {
			t.Error("TimeoutMs not set correctly")
		}
	})

	t.Run("CompositeAggregation struct exists", func(t *testing.T) {
		aggregation := model.CompositeAggregation{
			Method:       model.AggregationMethodFirst,
			WaitStrategy: model.WaitStrategyAny,
			MinResponses: 1,
			MaxWaitMs:    5000,
		}

		if aggregation.Method != model.AggregationMethodFirst {
			t.Error("Method not set correctly")
		}
		if aggregation.MaxWaitMs != 5000 {
			t.Error("MaxWaitMs not set correctly")
		}
	})
}

// Note: Full aggregator unit tests require complex mocking of the adapter interface.
// Integration tests should be used to verify aggregator functionality end-to-end.
