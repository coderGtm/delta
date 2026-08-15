package attendance

import (
	"math"
	"testing"
)

func TestIsWithinRadiusMeters(t *testing.T) {
	// Approx 111 m north of the origin is within a 200 m radius.
	if !IsWithinRadiusMeters(0, 0, 0.001, 0, 200) {
		t.Error("expected approx 111 m offset to be within 200 m radius")
	}

	// Approx 11.1 km north of the origin is outside a 100 m radius.
	if IsWithinRadiusMeters(0, 0, 0.1, 0, 100) {
		t.Error("expected approx 11.1 km offset to be outside 100 m radius")
	}

	// A point identical to the center is within any positive radius.
	if !IsWithinRadiusMeters(12.34, 56.78, 12.34, 56.78, 1) {
		t.Error("expected same point to be within any radius")
	}

	// Boundary: a latitude offset of 180*5000/(R*pi) degrees yields a computed
	// distance of exactly 5000 m, so a radius of 5000 exercises the inclusive
	// <= comparison at the exact boundary.
	theta := 180.0 * 5000.0 / (earthRadiusMeters * math.Pi)
	if got := distanceMeters(0, 0, theta, 0); got != 5000.0 {
		t.Fatalf("expected boundary distance of 5000 m, got %f", got)
	}
	if !IsWithinRadiusMeters(0, 0, theta, 0, 5000) {
		t.Error("expected point exactly at the radius to be within")
	}
	if IsWithinRadiusMeters(0, 0, theta, 0, 4999) {
		t.Error("expected point beyond a 4999 m radius to be outside")
	}

	// Approx 111 km (1 degree of latitude) is outside a 10 km radius.
	if IsWithinRadiusMeters(0, 0, 1, 0, 10000) {
		t.Error("expected approx 111 km offset to be outside 10 km radius")
	}

	// Longitudes on either side of the date line are close together; the
	// great-circle distance (~22 km) is within a generous radius.
	if !IsWithinRadiusMeters(0, 179.9, 0, -179.9, 100_000) {
		t.Error("expected cross-date-line points to be within a generous radius")
	}

	// A point identical to the center with radius 0 (distance 0) is within.
	if !IsWithinRadiusMeters(45.0, -70.0, 45.0, -70.0, 0) {
		t.Error("expected same point with radius 0 to be within")
	}
}
