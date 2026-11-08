package config

import (
	"reflect"
	"testing"
)

func TestSplitListTrimsAndDropsEmptyEntries(t *testing.T) {
	got := splitList(" acme/a@0123456789abcdef0123456789abcdef01234567 , ,, other/b@fedcba9876543210fedcba9876543210fedcba98 ")
	want := []string{
		"acme/a@0123456789abcdef0123456789abcdef01234567",
		"other/b@fedcba9876543210fedcba9876543210fedcba98",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitList() = %#v, want %#v", got, want)
	}
}

func TestSplitListEmptyDisablesFeature(t *testing.T) {
	if got := splitList(" \t "); got != nil {
		t.Fatalf("expected nil for blank value, got %#v", got)
	}
}
