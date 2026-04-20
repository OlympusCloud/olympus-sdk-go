package olympus

import (
	"sync"
	"testing"
)

// TestHasScopeBit_ConcurrentSafe verifies that the cache fields behind
// HasScopeBit are race-free under concurrent access. Run with -race.
func TestHasScopeBit_ConcurrentSafe(t *testing.T) {
	oc := testClient(t, "http://ignored")
	bitset := makeBitset([]int{0, 7, 63, 128, 512, 1023}, 128)
	token := makeJWT(map[string]interface{}{
		"sub": "u", "tenant_id": "t", "session_id": "s",
		"roles":             []string{},
		"app_id":            "pizza-os",
		"app_scopes_bitset": bitset,
		"iat": 0, "exp": 9999999999, "iss": "i", "aud": "a",
	})
	oc.SetAccessToken(token)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < 1000; j++ {
				_ = oc.HasScopeBit(0)
				_ = oc.HasScopeBit(7)
				_ = oc.HasScopeBit(63)
				_ = oc.HasScopeBit(128)
				_ = oc.HasScopeBit(512)
				_ = oc.HasScopeBit(1023)
				_ = oc.IsAppScoped()
			}
		}()
	}
	close(start)
	wg.Wait()

	// Final sanity check.
	if !oc.HasScopeBit(0) {
		t.Error("cache corrupted during concurrent access")
	}
	if !oc.HasScopeBit(1023) {
		t.Error("cache corrupted during concurrent access (bit 1023)")
	}
}

// TestConcurrentTokenSwitch verifies that rapid SetAccessToken calls from
// multiple goroutines don't corrupt the cache.
func TestConcurrentTokenSwitch(t *testing.T) {
	oc := testClient(t, "http://ignored")
	tokenA := makeJWT(map[string]interface{}{
		"sub": "u", "tenant_id": "t", "session_id": "s", "roles": []string{},
		"app_id": "a", "app_scopes_bitset": makeBitset([]int{0}, 128),
		"iat": 0, "exp": 9999999999, "iss": "i", "aud": "a",
	})
	tokenB := makeJWT(map[string]interface{}{
		"sub": "u", "tenant_id": "t", "session_id": "s", "roles": []string{},
		"app_id": "b", "app_scopes_bitset": makeBitset([]int{5}, 128),
		"iat": 0, "exp": 9999999999, "iss": "i", "aud": "a",
	})

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(odd bool) {
			defer wg.Done()
			<-start
			for j := 0; j < 500; j++ {
				if j%2 == 0 {
					oc.SetAccessToken(tokenA)
				} else {
					oc.SetAccessToken(tokenB)
				}
				_ = oc.HasScopeBit(0)
				_ = oc.HasScopeBit(5)
			}
		}(i%2 == 0)
	}
	close(start)
	wg.Wait()
}
