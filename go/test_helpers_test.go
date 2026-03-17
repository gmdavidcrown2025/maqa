package maqa

import (
	"math"
	"testing"
)

func assertAlmostEqual(t *testing.T, actual float64, expected float64, tolerance float64) {
	t.Helper()
	if math.Abs(actual-expected) > tolerance {
		t.Fatalf("expected %.12f, got %.12f", expected, actual)
	}
}
