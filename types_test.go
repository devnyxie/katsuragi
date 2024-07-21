package katsuragi

import "testing"

//NewFetcher: no props
func TestNewFetcher(t *testing.T) {
	f := NewFetcher(nil)
	if f.props.Timeout != 3000000000 {
		t.Errorf("Expected default timeout to be 3000000000, got %d", f.props.Timeout)
	}
	if f.props.CacheCap != 10 {
		t.Errorf("Expected default cache capacity to be 10, got %d", f.props.CacheCap)
	}
}

//NewFetcher: with zero props
func TestNewFetcher_ZeroProps(t *testing.T) {
	f := NewFetcher(&FetcherProps{Timeout: 0, CacheCap: 0})
	if f.props.Timeout != 3000000000 {
		t.Errorf("Expected default timeout to be 3000000000, got %d", f.props.Timeout)
	}
	if f.props.CacheCap != 10 {
		t.Errorf("Expected default cache capacity to be 10, got %d", f.props.CacheCap)
	}
}