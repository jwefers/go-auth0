package management

import (
	"fmt"
	"sync"
	"testing"

	"github.com/auth0/go-auth0/internal/client"
)

// TestNewDoesNotShareClientInfo checks that each Management client is seeded
// with a freshly built client.DefaultAuth0ClientInfo(), so
// WithAuth0ClientEnvEntry neither leaks env entries between clients nor
// affects the values handed to later clients.
func TestNewDoesNotShareClientInfo(t *testing.T) {
	m1, err := New("go-auth0-dev.eu.auth0.com", WithAuth0ClientEnvEntry("test-entry", "one"))
	if err != nil {
		t.Fatalf("cannot create management client: %v", err)
	}
	m2, err := New("go-auth0-dev.eu.auth0.com", WithAuth0ClientEnvEntry("test-entry", "two"))
	if err != nil {
		t.Fatalf("cannot create management client: %v", err)
	}

	if m1.auth0ClientInfo == m2.auth0ClientInfo {
		t.Fatal("two management clients share the same Auth0ClientInfo pointer")
	}
	m1.auth0ClientInfo.Env["probe"] = "m1"
	if _, leaked := m2.auth0ClientInfo.Env["probe"]; leaked {
		t.Fatal("mutating the Env map of one client leaked into another client's Auth0ClientInfo")
	}
	if _, leaked := client.DefaultAuth0ClientInfo().Env["probe"]; leaked {
		t.Fatal("mutating a client's Env map leaked into DefaultAuth0ClientInfo")
	}
	if got := m1.auth0ClientInfo.Env["test-entry"]; got != "one" {
		t.Fatalf("first client has test-entry=%q, want %q", got, "one")
	}
	if got := m2.auth0ClientInfo.Env["test-entry"]; got != "two" {
		t.Fatalf("second client has test-entry=%q, want %q", got, "two")
	}
	if got := m1.auth0ClientInfo.Env["go"]; got == "" {
		t.Fatal("default env entry \"go\" was lost from the client's copy")
	}
}

// TestNewConcurrentClientInfo runs many New calls concurrently, as happens
// when an application constructs clients for different tenants from several
// goroutines. Without a fresh client info per call this races on, and
// intermittently crashes with Go's fatal "concurrent map writes", inside
// WithAuth0ClientEnvEntry. Run with:
//
//	go test -race ./management/ -run TestNewConcurrentClientInfo
func TestNewConcurrentClientInfo(t *testing.T) {
	const clients = 64

	var wg sync.WaitGroup
	errCh := make(chan error, clients)
	for i := 0; i < clients; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			m, err := New(
				fmt.Sprintf("go-auth0-dev-%d.eu.auth0.com", i),
				WithAuth0ClientEnvEntry("test-entry", fmt.Sprintf("client-%d", i)),
			)
			if err != nil {
				errCh <- fmt.Errorf("client %d: %w", i, err)
				return
			}
			if got, want := m.auth0ClientInfo.Env["test-entry"], fmt.Sprintf("client-%d", i); got != want {
				errCh <- fmt.Errorf("client %d has test-entry=%q, want %q", i, got, want)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Error(err)
	}
	if got := client.DefaultAuth0ClientInfo().Env["test-entry"]; got != "" {
		t.Errorf("DefaultAuth0ClientInfo() leaked test-entry=%q from WithAuth0ClientEnvEntry across calls", got)
	}
}
