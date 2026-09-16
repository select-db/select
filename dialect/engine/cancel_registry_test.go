package engine

import "testing"

func TestRegisterCancel_RunningAgainCancelsTheEarlierRun(t *testing.T) {
	const key = "db:x:file:y"
	var first, second bool

	unregisterFirst := RegisterCancel(key, func() { first = true })
	unregisterSecond := RegisterCancel(key, func() { second = true })
	defer unregisterSecond()

	if !first {
		t.Fatal("registering again did not cancel the earlier run")
	}

	// The earlier run finishing late must not take the newer run's cancel.
	unregisterFirst()
	Cancel(key)
	if !second {
		t.Fatal("the newer run could not be cancelled after the earlier one finished")
	}
}
