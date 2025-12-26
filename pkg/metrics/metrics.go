package metrics

import (
	"sync"
	"time"
)

// Metrics holds application metrics
type Metrics struct {
	mu sync.RWMutex

	// Chat metrics
	TotalMessages         int64
	TotalConversations    int64
	TotalPendingActions   int64
	TotalConfirmedActions int64
	TotalCancelledActions int64
	TotalExpiredActions   int64
	TotalFailedActions    int64

	// Latency tracking
	MessageLatencyTotal time.Duration
	MessageLatencyCount int64
	ActionLatencyTotal  time.Duration
	ActionLatencyCount  int64

	// Error tracking
	TotalErrors      int64
	ValidationErrors int64
	PolicyViolations int64
	ConflictErrors   int64

	// Rate tracking
	LastResetTime     time.Time
	MessagesPerMinute float64
	ActionsPerMinute  float64
}

var (
	instance *Metrics
	once     sync.Once
)

// GetInstance returns the singleton metrics instance
func GetInstance() *Metrics {
	once.Do(func() {
		instance = &Metrics{
			LastResetTime: time.Now(),
		}
	})
	return instance
}

// IncrementMessages increments the message count
func (m *Metrics) IncrementMessages() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalMessages++
	m.updateRate()
}

// IncrementConversations increments the conversation count
func (m *Metrics) IncrementConversations() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalConversations++
}

// IncrementPendingActions increments the pending action count
func (m *Metrics) IncrementPendingActions() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalPendingActions++
	m.updateRate()
}

// IncrementConfirmedActions increments the confirmed action count
func (m *Metrics) IncrementConfirmedActions() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalConfirmedActions++
}

// IncrementCancelledActions increments the canceled action count
func (m *Metrics) IncrementCancelledActions() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalCancelledActions++
}

// IncrementExpiredActions increments the expired action count
func (m *Metrics) IncrementExpiredActions() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalExpiredActions++
}

// IncrementFailedActions increments the failed action count
func (m *Metrics) IncrementFailedActions() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalFailedActions++
}

// RecordMessageLatency records message processing latency
func (m *Metrics) RecordMessageLatency(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MessageLatencyTotal += latency
	m.MessageLatencyCount++
}

// RecordActionLatency records action execution latency
func (m *Metrics) RecordActionLatency(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ActionLatencyTotal += latency
	m.ActionLatencyCount++
}

// IncrementErrors increments the total error count
func (m *Metrics) IncrementErrors() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalErrors++
}

// IncrementValidationErrors increments validation error count
func (m *Metrics) IncrementValidationErrors() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ValidationErrors++
}

// IncrementPolicyViolations increments policy violation count
func (m *Metrics) IncrementPolicyViolations() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PolicyViolations++
}

// IncrementConflictErrors increments conflict error count
func (m *Metrics) IncrementConflictErrors() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ConflictErrors++
}

// GetAverageMessageLatency returns the average message latency
func (m *Metrics) GetAverageMessageLatency() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.MessageLatencyCount == 0 {
		return 0
	}
	return m.MessageLatencyTotal / time.Duration(m.MessageLatencyCount)
}

// GetAverageActionLatency returns the average action latency
func (m *Metrics) GetAverageActionLatency() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ActionLatencyCount == 0 {
		return 0
	}
	return m.ActionLatencyTotal / time.Duration(m.ActionLatencyCount)
}

// GetConfirmationRate returns the percentage of confirmed actions
func (m *Metrics) GetConfirmationRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	total := m.TotalConfirmedActions + m.TotalCancelledActions + m.TotalExpiredActions
	if total == 0 {
		return 0
	}
	return float64(m.TotalConfirmedActions) / float64(total) * 100
}

// GetSuccessRate returns the percentage of successful actions
func (m *Metrics) GetSuccessRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	total := m.TotalConfirmedActions + m.TotalFailedActions
	if total == 0 {
		return 0
	}
	return float64(m.TotalConfirmedActions) / float64(total) * 100
}

// updateRate updates per-minute rates based on cumulative averages (should be called while holding lock)
// Note: This calculates average rates since the last reset, not instantaneous rates.
// For true per-minute rates, a sliding window approach would be needed.
func (m *Metrics) updateRate() {
	elapsed := time.Since(m.LastResetTime).Minutes()
	if elapsed > 0 {
		m.MessagesPerMinute = float64(m.TotalMessages) / elapsed
		m.ActionsPerMinute = float64(m.TotalPendingActions) / elapsed
	}
}

// GetSnapshot returns a snapshot of current metrics
func (m *Metrics) GetSnapshot() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"total_messages":          m.TotalMessages,
		"total_conversations":     m.TotalConversations,
		"total_pending_actions":   m.TotalPendingActions,
		"total_confirmed_actions": m.TotalConfirmedActions,
		"total_cancelled_actions": m.TotalCancelledActions, //nolint:misspell
		"total_expired_actions":   m.TotalExpiredActions,
		"total_failed_actions":    m.TotalFailedActions,
		"avg_message_latency_ms": func() int64 {
			if m.MessageLatencyCount == 0 {
				return 0
			}
			return (m.MessageLatencyTotal / time.Duration(m.MessageLatencyCount)).Milliseconds()
		}(),
		"avg_action_latency_ms": func() int64 {
			if m.ActionLatencyCount == 0 {
				return 0
			}
			return (m.ActionLatencyTotal / time.Duration(m.ActionLatencyCount)).Milliseconds()
		}(),
		"confirmation_rate_pct": m.GetConfirmationRate(),
		"success_rate_pct":      m.GetSuccessRate(),
		"total_errors":          m.TotalErrors,
		"validation_errors":     m.ValidationErrors,
		"policy_violations":     m.PolicyViolations,
		"conflict_errors":       m.ConflictErrors,
		"messages_per_minute":   m.MessagesPerMinute,
		"actions_per_minute":    m.ActionsPerMinute,
	}
}

// Reset resets all metrics
func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalMessages = 0
	m.TotalConversations = 0
	m.TotalPendingActions = 0
	m.TotalConfirmedActions = 0
	m.TotalCancelledActions = 0
	m.TotalExpiredActions = 0
	m.TotalFailedActions = 0
	m.MessageLatencyTotal = 0
	m.MessageLatencyCount = 0
	m.ActionLatencyTotal = 0
	m.ActionLatencyCount = 0
	m.TotalErrors = 0
	m.ValidationErrors = 0
	m.PolicyViolations = 0
	m.ConflictErrors = 0
	m.LastResetTime = time.Now()
	m.MessagesPerMinute = 0
	m.ActionsPerMinute = 0
}
