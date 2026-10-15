// Package bank models bank accounts that are safe for concurrent use.
//
// Balances are stored in cents as int64 to avoid floating-point errors.
package bank

import (
	"errors"
	"math"
	"sync"
	"sync/atomic"
)

// Errors returned by account operations.
var (
	ErrInvalidAmount     = errors.New("bank: amount must be positive")
	ErrInsufficientFunds = errors.New("bank: insufficient funds")
	ErrSameAccount       = errors.New("bank: cannot transfer to the same account")
	ErrOverflow          = errors.New("bank: balance overflow")
)

// nextSeq hands out a unique, increasing number to each account. It gives
// accounts a global order, which Transfer uses to lock them without deadlock.
var nextSeq atomic.Uint64

// Account is a bank account. Create accounts with NewAccount.
type Account struct {
	seq     uint64
	owner   string
	mu      sync.Mutex
	balance int64 // cents; guarded by mu
}

// NewAccount opens an account with the given initial balance in cents.
func NewAccount(owner string, initialCents int64) (*Account, error) {
	if initialCents < 0 {
		return nil, ErrInvalidAmount
	}
	return &Account{seq: nextSeq.Add(1), owner: owner, balance: initialCents}, nil
}

// Owner returns the account holder's name.
func (a *Account) Owner() string { return a.owner }

// Balance returns the current balance in cents.
func (a *Account) Balance() int64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.balance
}

// Deposit adds cents to the balance.
func (a *Account) Deposit(cents int64) error {
	if cents <= 0 {
		return ErrInvalidAmount
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.balance > math.MaxInt64-cents {
		return ErrOverflow
	}
	a.balance += cents
	return nil
}

// Withdraw removes cents from the balance, failing with ErrInsufficientFunds
// if the account does not have enough money.
func (a *Account) Withdraw(cents int64) error {
	if cents <= 0 {
		return ErrInvalidAmount
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.balance < cents {
		return ErrInsufficientFunds
	}
	a.balance -= cents
	return nil
}

// Transfer moves cents from one account to another atomically: other
// goroutines never observe the money missing from both accounts.
//
// Both mutexes are acquired in a fixed order (by account sequence number), so
// two opposite transfers running at the same time cannot deadlock.
func Transfer(from, to *Account, cents int64) error {
	if from == to {
		return ErrSameAccount
	}
	if cents <= 0 {
		return ErrInvalidAmount
	}
	first, second := from, to
	if first.seq > second.seq {
		first, second = second, first
	}
	first.mu.Lock()
	defer first.mu.Unlock()
	second.mu.Lock()
	defer second.mu.Unlock()

	if from.balance < cents {
		return ErrInsufficientFunds
	}
	if to.balance > math.MaxInt64-cents {
		return ErrOverflow
	}
	from.balance -= cents
	to.balance += cents
	return nil
}
