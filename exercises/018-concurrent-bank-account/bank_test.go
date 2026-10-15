package bank

import (
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"testing"
)

func newAccount(t *testing.T, owner string, cents int64) *Account {
	t.Helper()
	a, err := NewAccount(owner, cents)
	if err != nil {
		t.Fatalf("NewAccount(%q, %d): %v", owner, cents, err)
	}
	return a
}

func TestAccountOperations(t *testing.T) {
	tests := []struct {
		name        string
		start       int64
		op          func(a *Account) error
		wantErr     error
		wantBalance int64
	}{
		{"deposit", 100, func(a *Account) error { return a.Deposit(50) }, nil, 150},
		{"withdraw", 100, func(a *Account) error { return a.Withdraw(40) }, nil, 60},
		{"withdraw everything", 100, func(a *Account) error { return a.Withdraw(100) }, nil, 0},
		{"overdraft", 100, func(a *Account) error { return a.Withdraw(101) }, ErrInsufficientFunds, 100},
		{"zero deposit", 100, func(a *Account) error { return a.Deposit(0) }, ErrInvalidAmount, 100},
		{"negative withdraw", 100, func(a *Account) error { return a.Withdraw(-5) }, ErrInvalidAmount, 100},
		{"deposit overflow", math.MaxInt64, func(a *Account) error { return a.Deposit(1) }, ErrOverflow, math.MaxInt64},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newAccount(t, "alice", tt.start)
			if err := tt.op(a); !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if got := a.Balance(); got != tt.wantBalance {
				t.Errorf("Balance() = %d, want %d", got, tt.wantBalance)
			}
		})
	}
}

func TestNewAccountRejectsNegativeBalance(t *testing.T) {
	if _, err := NewAccount("bob", -1); !errors.Is(err, ErrInvalidAmount) {
		t.Errorf("NewAccount() error = %v, want ErrInvalidAmount", err)
	}
}

func TestTransfer(t *testing.T) {
	tests := []struct {
		name               string
		fromStart, toStart int64
		amount             int64
		sameAccount        bool
		wantErr            error
		wantFrom, wantTo   int64
	}{
		{name: "regular", fromStart: 100, toStart: 5, amount: 30, wantFrom: 70, wantTo: 35},
		{name: "all the money", fromStart: 100, toStart: 0, amount: 100, wantFrom: 0, wantTo: 100},
		{name: "insufficient funds", fromStart: 10, toStart: 0, amount: 11, wantErr: ErrInsufficientFunds, wantFrom: 10, wantTo: 0},
		{name: "invalid amount", fromStart: 10, toStart: 0, amount: 0, wantErr: ErrInvalidAmount, wantFrom: 10, wantTo: 0},
		{name: "receiver overflow", fromStart: 10, toStart: math.MaxInt64, amount: 1, wantErr: ErrOverflow, wantFrom: 10, wantTo: math.MaxInt64},
		{name: "same account", fromStart: 10, amount: 1, sameAccount: true, wantErr: ErrSameAccount, wantFrom: 10, wantTo: 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from := newAccount(t, "from", tt.fromStart)
			to := newAccount(t, "to", tt.toStart)
			if tt.sameAccount {
				to = from
			}
			if err := Transfer(from, to, tt.amount); !errors.Is(err, tt.wantErr) {
				t.Fatalf("Transfer() error = %v, want %v", err, tt.wantErr)
			}
			if from.Balance() != tt.wantFrom || to.Balance() != tt.wantTo {
				t.Errorf("balances = (%d, %d), want (%d, %d)", from.Balance(), to.Balance(), tt.wantFrom, tt.wantTo)
			}
		})
	}
}

func TestConcurrentDepositsAndWithdrawals(t *testing.T) {
	a := newAccount(t, "alice", 1000)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if err := a.Deposit(10); err != nil {
				t.Errorf("Deposit: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			if err := a.Withdraw(5); err != nil {
				t.Errorf("Withdraw: %v", err)
			}
		}()
	}
	wg.Wait()
	if got, want := a.Balance(), int64(1000+100*10-100*5); got != want {
		t.Errorf("Balance() = %d, want %d", got, want)
	}
}

func TestConcurrentWithdrawalsNeverOverdraw(t *testing.T) {
	a := newAccount(t, "alice", 500)
	var succeeded atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := a.Withdraw(10); err == nil {
				succeeded.Add(1)
			} else if !errors.Is(err, ErrInsufficientFunds) {
				t.Errorf("Withdraw: %v", err)
			}
		}()
	}
	wg.Wait()
	if succeeded.Load() != 50 || a.Balance() != 0 {
		t.Errorf("succeeded = %d, balance = %d; want 50 and 0", succeeded.Load(), a.Balance())
	}
}

// Opposite transfers between the same two accounts would deadlock if the locks
// were taken in argument order; they must finish and conserve the total.
func TestOppositeTransfersDoNotDeadlock(t *testing.T) {
	a := newAccount(t, "alice", 10_000)
	b := newAccount(t, "bob", 10_000)
	var wg sync.WaitGroup
	for i := 0; i < 500; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = Transfer(a, b, 7)
		}()
		go func() {
			defer wg.Done()
			_ = Transfer(b, a, 3)
		}()
	}
	wg.Wait()
	if total := a.Balance() + b.Balance(); total != 20_000 {
		t.Errorf("total = %d, want 20000", total)
	}
	if want := int64(10_000 - 500*7 + 500*3); a.Balance() != want {
		t.Errorf("alice = %d, want %d", a.Balance(), want)
	}
}
