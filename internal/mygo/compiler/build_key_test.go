package compiler

import (
	"strings"
	"testing"
)

func TestBuildCacheKeyIsDeterministicAndOrderIndependent(t *testing.T) {
	first := BuildCacheKey("source", []string{"dep-b", "dep-a"}, 0)
	second := BuildCacheKey("source", []string{"dep-a", "dep-b"}, 0)
	if first != second {
		t.Fatal("cache key depends on imported digest ordering")
	}
	if first != BuildCacheKey("source", []string{"dep-a", "dep-b"}, 0) {
		t.Fatal("cache key is not deterministic")
	}
}

func TestCheckRepresentationVersionRejectsMixedBuilds(t *testing.T) {
	if err := CheckRepresentationVersion(RepresentationVersion); err != nil {
		t.Fatalf("current representation version was rejected: %v", err)
	}
	err := CheckRepresentationVersion(RepresentationVersion + 1)
	if err == nil {
		t.Fatal("mixed representation versions were accepted")
	}
	if !strings.Contains(err.Error(), "representation version") {
		t.Fatalf("compatibility diagnostic is unclear: %v", err)
	}
}

func TestBuildCacheKeyChangesWithCompilerInputs(t *testing.T) {
	base := buildCacheKeyFor(RepresentationVersion, "source", []string{"dep"}, 0)
	if base == buildCacheKeyFor(RepresentationVersion+1, "source", []string{"dep"}, 0) {
		t.Fatal("cache key did not change with the representation version")
	}
	if base == buildCacheKeyFor(RepresentationVersion, "source", []string{"dep"}, 1) {
		t.Fatal("cache key did not change with the optimization level")
	}
	if base == buildCacheKeyFor(RepresentationVersion, "source", []string{"other"}, 0) {
		t.Fatal("cache key did not change with an imported source digest")
	}
	if base == buildCacheKeyFor(RepresentationVersion, "other", []string{"dep"}, 0) {
		t.Fatal("cache key did not change with the source digest")
	}
}
