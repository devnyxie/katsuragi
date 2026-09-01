package katsuragi

import "testing"

// NewFetcher: nil props
func TestNewFetcher_NilProps(t *testing.T) {

	f := NewFetcher(nil)
	if f.props.Timeout != 3000000000 {
		t.Errorf("Expected default timeout to be 3000000000, got %d", f.props.Timeout)
	}
	if f.props.CacheCap != 10 {
		t.Errorf("Expected default cache capacity to be 10, got %d", f.props.CacheCap)
	}
}

// NewFetcher: with zero props
func TestNewFetcher_ZeroProps(t *testing.T) {
	f := NewFetcher(&FetcherProps{Timeout: 0, CacheCap: 0})
	if f.props.Timeout != 3000000000 {
		t.Errorf("Expected default timeout to be 3000000000, got %d", f.props.Timeout)
	}
	if f.props.CacheCap != 10 {
		t.Errorf("Expected default cache capacity to be 10, got %d", f.props.CacheCap)
	}
}

// NewFetcher: brand-resolution defaults
func TestNewFetcher_BrandDefaults(t *testing.T) {
	f := NewFetcher(nil)

	if _, ok := f.props.SearchProvider.(*DuckDuckGoSearchProvider); !ok {
		t.Errorf("Expected default SearchProvider to be *DuckDuckGoSearchProvider, got %T", f.props.SearchProvider)
	}
	if _, ok := f.props.BrandCache.(*InMemoryBrandCache); !ok {
		t.Errorf("Expected default BrandCache to be *InMemoryBrandCache, got %T", f.props.BrandCache)
	}
	if f.props.BrandGuessTimeout != defaultBrandGuessTimeout {
		t.Errorf("Expected default BrandGuessTimeout to be %s, got %s", defaultBrandGuessTimeout, f.props.BrandGuessTimeout)
	}
	if f.props.BrandSearchTimeout != defaultBrandSearchTimeout {
		t.Errorf("Expected default BrandSearchTimeout to be %s, got %s", defaultBrandSearchTimeout, f.props.BrandSearchTimeout)
	}
	if f.props.GuessDomains == nil {
		t.Errorf("Expected default GuessDomains to be set, got nil")
	} else {
		got := f.props.GuessDomains("allegro", "pl")
		want := []string{"https://allegro.com", "https://allegro.pl"}
		if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
			t.Errorf("Expected default GuessDomains(\"allegro\", \"pl\") to be %v, got %v", want, got)
		}
	}
}
