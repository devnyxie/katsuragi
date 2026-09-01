package katsuragi

import (
	"testing"
	"time"
)

func TestInMemoryBrandCache_GetSetRoundTrip(t *testing.T) {
	c := &InMemoryBrandCache{}

	if _, ok := c.Get("allegro"); ok {
		t.Fatalf("expected no entry before Set")
	}

	c.Set("allegro", "https://allegro.pl")
	domain, ok := c.Get("allegro")
	if !ok || domain != "https://allegro.pl" {
		t.Fatalf("expected (https://allegro.pl, true), got (%q, %v)", domain, ok)
	}

	// Set again replaces the value.
	c.Set("allegro", "https://allegro.com")
	domain, ok = c.Get("allegro")
	if !ok || domain != "https://allegro.com" {
		t.Fatalf("expected updated value https://allegro.com, got (%q, %v)", domain, ok)
	}
}

func TestInMemoryBrandCache_EvictsLeastRecentlyUsedAtCapacity(t *testing.T) {
	c := &InMemoryBrandCache{Capacity: 2}

	c.Set("a", "https://a.com")
	c.Set("b", "https://b.com")
	// Touch "a" so "b" becomes the least-recently-used entry.
	if _, ok := c.Get("a"); !ok {
		t.Fatalf("expected a to be present")
	}
	c.Set("c", "https://c.com") // should evict "b", not "a"

	if _, ok := c.Get("b"); ok {
		t.Fatalf("expected b to have been evicted")
	}
	if _, ok := c.Get("a"); !ok {
		t.Fatalf("expected a to still be present")
	}
	if _, ok := c.Get("c"); !ok {
		t.Fatalf("expected c to be present")
	}
}

func TestInMemoryBrandCache_TTLExpiry(t *testing.T) {
	c := &InMemoryBrandCache{TTL: 30 * time.Millisecond}
	c.Set("allegro", "https://allegro.pl")

	if _, ok := c.Get("allegro"); !ok {
		t.Fatalf("expected entry to be present immediately after Set")
	}

	time.Sleep(60 * time.Millisecond)

	if _, ok := c.Get("allegro"); ok {
		t.Fatalf("expected entry to have expired after TTL")
	}
}
