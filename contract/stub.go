package contract

import (
	"context"
	"fmt"
	"sync"

	"github.com/coderGtm/delta/auth"
)

// StubFirebase is a token-to-profile Firebase implementation used by the
// contract test suite. VerifyIDToken resolves the well-known test tokens to
// their profiles and rejects unknown tokens; DeleteUser records the UIDs it
// was asked to delete.
type StubFirebase struct {
	mu       sync.Mutex
	profiles map[string]*auth.UserInfo
	// Deleted collects the UIDs passed to DeleteUser, in call order.
	Deleted []string
}

// newStubFirebase returns a StubFirebase with the owner and employee test
// profiles wired to their fixed tokens.
func newStubFirebase() *StubFirebase {
	return &StubFirebase{
		profiles: map[string]*auth.UserInfo{
			"token-owner":  {UID: "owner-uid", Name: "Owner User", Email: "owner@example.com", PhoneNumber: ""},
			"token-emp":    {UID: "emp-uid", Name: "Employee User", Email: "employee@example.com", PhoneNumber: ""},
			"token-second": {UID: "second-uid", Name: "Second Employee", Email: "second@example.com", PhoneNumber: ""},
		},
	}
}

// VerifyIDToken returns the profile associated with token, or an error for a
// token that is not one of the well-known test tokens.
func (s *StubFirebase) VerifyIDToken(ctx context.Context, token string) (*auth.UserInfo, error) {
	if info, ok := s.profiles[token]; ok {
		return info, nil
	}
	return nil, fmt.Errorf("unknown token %q", token)
}

// DeleteUser records uid in Deleted and always succeeds.
func (s *StubFirebase) DeleteUser(ctx context.Context, uid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Deleted = append(s.Deleted, uid)
	return nil
}
