package datasource

import (
	"fmt"
	"testing"
	"time"
)

func TestCellarTokens_RenewsBeforeExpiryUnderSteadyUse(t *testing.T) {
	clock := time.Unix(0, 0)
	signed := 0
	tk := newCellarTokens()
	tk.now = func() time.Time { return clock }
	tk.sign = func(time.Duration) (string, error) {
		signed++
		return fmt.Sprint("tok-", signed), nil
	}

	// A request every 10s for 120s: a sign every reuseFor, never one per request.
	var last string
	for range 12 {
		tok, err := tk.Token()
		if err != nil {
			t.Fatal(err)
		}
		last = tok
		clock = clock.Add(10 * time.Second)
	}
	if signed != 3 || last != "tok-3" {
		t.Fatalf("got %d signs, last %q; want 3 signs, last tok-3", signed, last)
	}
}
