package gatepaths

import (
	"reflect"
	"testing"
)

func TestNormalize(t *testing.T) {
	got, err := Normalize([]string{" /pricing ", "", "/docs/*", "/pricing"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"/pricing", "/docs/*"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
	for _, bad := range []string{"pricing", "/a b", "/a?x=1", "/a*b", "/a/../b", "/a//b", "/a/./b", `/a\b`} {
		if _, err := Normalize([]string{bad}); err == nil {
			t.Errorf("Normalize(%q) should fail", bad)
		}
	}
}

func TestMatch(t *testing.T) {
	pats := []string{"/", "/pricing", "/docs/*", "/assets*"}
	yes := []string{"/", "/pricing", "/docs/", "/docs/a/b", "/assets", "/assets/app.js", "/assetsx"}
	no := []string{"/pricing/", "/pricing2", "/docs", "/admin", "/docs/../admin", "/docs//x", "/docs/./x", "/x/../pricing", `/docs/..\admin`, ""}
	for _, p := range yes {
		if !Match(pats, p) {
			t.Errorf("Match(%q) = false, want true", p)
		}
	}
	for _, p := range no {
		if Match(pats, p) {
			t.Errorf("Match(%q) = true, want false", p)
		}
	}
	if Match(nil, "/") {
		t.Error("no patterns must match nothing")
	}
}
