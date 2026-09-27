// backend/internal/services/client_health_test.go
package services

import "testing"

func TestClientHealthIsRedWhenBothSignalsFire(t *testing.T) {
	if got := ComputeClientHealth(true, true); got != "red" {
		t.Fatalf("expected red when both signals fire, got %s", got)
	}
}

func TestClientHealthIsYellowWhenOnlyStalledTaskFires(t *testing.T) {
	if got := ComputeClientHealth(true, false); got != "yellow" {
		t.Fatalf("expected yellow when only the stalled-task signal fires, got %s", got)
	}
}

func TestClientHealthIsYellowWhenOnlyOverdueInvoiceFires(t *testing.T) {
	if got := ComputeClientHealth(false, true); got != "yellow" {
		t.Fatalf("expected yellow when only the overdue-invoice signal fires, got %s", got)
	}
}

func TestClientHealthIsGreenWhenNeitherSignalFires(t *testing.T) {
	if got := ComputeClientHealth(false, false); got != "green" {
		t.Fatalf("expected green when neither signal fires, got %s", got)
	}
}
