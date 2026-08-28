//lint:file-ignore SA1019 Bridges to the deprecated AdapterCache, as the rest of this package does.

package captcha

import (
	"testing"

	"github.com/go-admin-team/go-admin-core/v2/storage/cache"
)

// The captcha endpoint is unauthenticated and sits in front of the login form,
// which makes it the one route an anonymous client can call freely. An
// end-to-end sweep put it at roughly 8k req/s while a plain framework route
// reached 147k, so it is worth knowing which part costs that.
//
// Generation renders a PNG; the store is a cache write. These separate them.

func benchStore(b *testing.B) {
	b.Helper()
	SetStore(NewCacheStore(cache.NewMemoryWithCleanupInterval(0), 600))
}

// BenchmarkDriverDigitGenerate is the digit captcha the login page uses:
// render, base64-encode and store the answer.
func BenchmarkDriverDigitGenerate(b *testing.B) {
	benchStore(b)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, _, err := DriverDigitFunc(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDriverStringGenerate is the string variant, which loads a font and
// so is not comparable to the digit driver.
func BenchmarkDriverStringGenerate(b *testing.B) {
	benchStore(b)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, _, err := DriverStringFunc(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDriverDigitGenerateParallel answers whether generation scales across
// cores or contends on the shared store.
func BenchmarkDriverDigitGenerateParallel(b *testing.B) {
	benchStore(b)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, _, _, err := DriverDigitFunc(); err != nil {
				b.Error(err)
				return
			}
		}
	})
}

// BenchmarkVerify is the login path: one store read plus a delete. It is the
// half that runs on every login attempt, generation being the half that runs
// when the form is displayed.
func BenchmarkVerify(b *testing.B) {
	c := cache.NewMemoryWithCleanupInterval(0)
	defer func() { _ = c.Close() }()
	store := NewCacheStore(c, 600)
	SetStore(store)

	// Verify with clear=false so the entry survives every iteration; clearing
	// would measure a miss after the first.
	if err := store.Set("bench-id", "1234"); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !Verify("bench-id", "1234", false) {
			b.Fatal("verification failed")
		}
	}
}
