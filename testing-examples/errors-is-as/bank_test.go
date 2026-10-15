package bank

import (
	"errors"
	"testing"
)

func newBank(t *testing.T) *Bank {
	t.Helper()
	b := New(1000)
	for id, opening := range map[string]int64{"alice": 800, "bob": 50} {
		if err := b.Open(id, opening); err != nil {
			t.Fatalf("Open(%q): %v", id, err)
		}
	}
	return b
}

// errors.Is walks the whole chain of wrapped errors, so the test keeps working
// when the production code later adds more context to the message.
func TestWithdrawSentinels(t *testing.T) {
	tests := []struct {
		name    string
		account string
		amount  int64
		want    error
	}{
		{"unknown account", "carol", 10, ErrAccountNotFound},
		{"balance too low", "bob", 100, ErrInsufficientFunds},
		{"above the limit", "alice", 5000, ErrLimitExceeded},
		{"ok", "alice", 100, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := newBank(t).Withdraw(tt.account, tt.amount)
			if !errors.Is(err, tt.want) {
				t.Errorf("Withdraw error = %v, want errors.Is(%v)", err, tt.want)
			}
		})
	}
}

// errors.As extracts a typed error from the chain to inspect its fields.
func TestWithdrawLimitError(t *testing.T) {
	err := newBank(t).Withdraw("alice", 5000)

	var limitErr *LimitError
	if !errors.As(err, &limitErr) {
		t.Fatalf("error %v is not a *LimitError", err)
	}
	if limitErr.Account != "alice" || limitErr.Limit != 1000 || limitErr.Requested != 5000 {
		t.Errorf("LimitError = %+v, unexpected fields", *limitErr)
	}
	// The custom Is method makes the typed error match the sentinel too.
	if !errors.Is(err, ErrLimitExceeded) {
		t.Error("errors.Is(err, ErrLimitExceeded) = false, want true")
	}
}

func TestTransferWrapsCause(t *testing.T) {
	b := newBank(t)
	err := b.Transfer("bob", "alice", 100)

	// The cause survives the extra "transfer bob -> alice" context.
	if !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("error = %v, want ErrInsufficientFunds in the chain", err)
	}
	// A plain == comparison is exactly what wrapping breaks:
	if err == ErrInsufficientFunds {
		t.Error("err == sentinel is true; the error was expected to be wrapped")
	}
	// The failed transfer must not change any balance.
	if bal, _ := b.Balance("alice"); bal != 800 {
		t.Errorf("alice balance = %d, want 800", bal)
	}
}

func TestTransferUnknownDestination(t *testing.T) {
	err := newBank(t).Transfer("alice", "nobody", 10)
	if !errors.Is(err, ErrAccountNotFound) {
		t.Errorf("error = %v, want ErrAccountNotFound", err)
	}
}

// errors.Join keeps every problem; errors.Is finds each of them.
func TestOpenJoinsValidationErrors(t *testing.T) {
	err := New(10).Open("", -5)
	for _, want := range []error{ErrEmptyID, ErrNegativeBalance} {
		if !errors.Is(err, want) {
			t.Errorf("error %v does not contain %v", err, want)
		}
	}
	if errors.Is(err, ErrAccountNotFound) {
		t.Error("error unexpectedly matches ErrAccountNotFound")
	}
	if err := New(10).Open("valid", 0); err != nil {
		t.Errorf("Open(valid) = %v, want nil", err)
	}
}
