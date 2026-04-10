package metrics

import (
	"testing"
	"time"
)

func newMetrics() *Metrics {
	return &Metrics{LastResetTime: time.Now()}
}

func TestIncrementMessages(t *testing.T) {
	m := newMetrics()
	m.IncrementMessages()
	m.IncrementMessages()
	if m.TotalMessages != 2 {
		t.Fatalf("expected TotalMessages=2, got %d", m.TotalMessages)
	}
}

func TestIncrementConversations(t *testing.T) {
	m := newMetrics()
	m.IncrementConversations()
	if m.TotalConversations != 1 {
		t.Fatalf("expected TotalConversations=1, got %d", m.TotalConversations)
	}
}

func TestIncrementPendingActions(t *testing.T) {
	m := newMetrics()
	m.IncrementPendingActions()
	m.IncrementPendingActions()
	if m.TotalPendingActions != 2 {
		t.Fatalf("expected TotalPendingActions=2, got %d", m.TotalPendingActions)
	}
}

func TestIncrementConfirmedCancelledExpiredFailed(t *testing.T) {
	m := newMetrics()
	m.IncrementConfirmedActions()
	m.IncrementCancelledActions()
	m.IncrementCancelledActions()
	m.IncrementExpiredActions()
	m.IncrementFailedActions()

	if m.TotalConfirmedActions != 1 {
		t.Fatalf("expected 1 confirmed, got %d", m.TotalConfirmedActions)
	}
	if m.TotalCancelledActions != 2 {
		t.Fatalf("expected 2 canceled, got %d", m.TotalCancelledActions)
	}
	if m.TotalExpiredActions != 1 {
		t.Fatalf("expected 1 expired, got %d", m.TotalExpiredActions)
	}
	if m.TotalFailedActions != 1 {
		t.Fatalf("expected 1 failed, got %d", m.TotalFailedActions)
	}
}

func TestIncrementErrors(t *testing.T) {
	m := newMetrics()
	m.IncrementErrors()
	m.IncrementValidationErrors()
	m.IncrementPolicyViolations()
	m.IncrementConflictErrors()

	if m.TotalErrors != 1 {
		t.Fatalf("expected TotalErrors=1, got %d", m.TotalErrors)
	}
	if m.ValidationErrors != 1 {
		t.Fatalf("expected ValidationErrors=1, got %d", m.ValidationErrors)
	}
	if m.PolicyViolations != 1 {
		t.Fatalf("expected PolicyViolations=1, got %d", m.PolicyViolations)
	}
	if m.ConflictErrors != 1 {
		t.Fatalf("expected ConflictErrors=1, got %d", m.ConflictErrors)
	}
}

func TestRecordMessageLatency(t *testing.T) {
	m := newMetrics()
	m.RecordMessageLatency(100 * time.Millisecond)
	m.RecordMessageLatency(200 * time.Millisecond)

	avg := m.GetAverageMessageLatency()
	if avg != 150*time.Millisecond {
		t.Fatalf("expected avg message latency 150ms, got %v", avg)
	}
}

func TestRecordActionLatency(t *testing.T) {
	m := newMetrics()
	m.RecordActionLatency(50 * time.Millisecond)
	m.RecordActionLatency(150 * time.Millisecond)

	avg := m.GetAverageActionLatency()
	if avg != 100*time.Millisecond {
		t.Fatalf("expected avg action latency 100ms, got %v", avg)
	}
}

func TestGetAverageLatency_ZeroCount(t *testing.T) {
	m := newMetrics()
	if m.GetAverageMessageLatency() != 0 {
		t.Fatal("expected zero average when no latency recorded")
	}
	if m.GetAverageActionLatency() != 0 {
		t.Fatal("expected zero average when no latency recorded")
	}
}

func TestGetConfirmationRate(t *testing.T) {
	m := newMetrics()
	// No actions yet — should be 0
	if m.GetConfirmationRate() != 0 {
		t.Fatal("expected 0 confirmation rate when no actions")
	}

	m.TotalConfirmedActions = 3
	m.TotalCancelledActions = 1
	m.TotalExpiredActions = 0

	rate := m.GetConfirmationRate()
	// 3 / (3+1+0) * 100 = 75
	if rate != 75.0 {
		t.Fatalf("expected confirmation rate 75.0, got %f", rate)
	}
}

func TestGetSuccessRate(t *testing.T) {
	m := newMetrics()
	if m.GetSuccessRate() != 0 {
		t.Fatal("expected 0 success rate when no actions")
	}

	m.TotalConfirmedActions = 4
	m.TotalFailedActions = 1

	rate := m.GetSuccessRate()
	// 4 / (4+1) * 100 = 80
	if rate != 80.0 {
		t.Fatalf("expected success rate 80.0, got %f", rate)
	}
}

func TestGetSnapshot_Keys(t *testing.T) {
	m := newMetrics()
	snap := m.GetSnapshot()

	requiredKeys := []string{
		"total_messages", "total_conversations", "total_pending_actions",
		"total_confirmed_actions", "total_canceled_actions", "total_expired_actions",
		"total_failed_actions", "avg_message_latency_ms", "avg_action_latency_ms",
		"confirmation_rate_pct", "success_rate_pct", "total_errors",
		"validation_errors", "policy_violations", "conflict_errors",
		"messages_per_minute", "actions_per_minute",
	}

	for _, key := range requiredKeys {
		if _, ok := snap[key]; !ok {
			t.Errorf("missing key %q in snapshot", key)
		}
	}
}

func TestGetSnapshot_AvgLatencyMs(t *testing.T) {
	m := newMetrics()
	m.RecordMessageLatency(200 * time.Millisecond)
	m.RecordActionLatency(400 * time.Millisecond)

	snap := m.GetSnapshot()
	if snap["avg_message_latency_ms"].(int64) != 200 {
		t.Fatalf("expected avg_message_latency_ms=200, got %v", snap["avg_message_latency_ms"])
	}
	if snap["avg_action_latency_ms"].(int64) != 400 {
		t.Fatalf("expected avg_action_latency_ms=400, got %v", snap["avg_action_latency_ms"])
	}
}

func TestReset(t *testing.T) {
	m := newMetrics()
	m.IncrementMessages()
	m.IncrementConversations()
	m.IncrementErrors()
	m.RecordMessageLatency(100 * time.Millisecond)

	m.Reset()

	if m.TotalMessages != 0 {
		t.Fatalf("expected TotalMessages=0 after reset, got %d", m.TotalMessages)
	}
	if m.TotalConversations != 0 {
		t.Fatalf("expected TotalConversations=0 after reset")
	}
	if m.TotalErrors != 0 {
		t.Fatalf("expected TotalErrors=0 after reset")
	}
	if m.MessageLatencyCount != 0 {
		t.Fatalf("expected MessageLatencyCount=0 after reset")
	}
	if m.GetAverageMessageLatency() != 0 {
		t.Fatalf("expected 0 avg latency after reset")
	}
}

func TestGetInstance_IsSingleton(t *testing.T) {
	a := GetInstance()
	b := GetInstance()
	if a != b {
		t.Fatal("GetInstance should return the same pointer each time")
	}
}
