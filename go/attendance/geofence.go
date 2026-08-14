// Package attendance provides attendance entry and geofence domain logic.
package attendance

import "math"

// earthRadiusMeters is the mean Earth radius in meters used by the geofence
// distance calculations.
const earthRadiusMeters = 6371000.0

// IsWithinRadiusMeters reports whether the request coordinates fall within
// radiusMeters of the center coordinates. The great-circle distance between
// the two points is compared against the radius; a point exactly at the
// boundary is considered within.
func IsWithinRadiusMeters(centerLat, centerLon, requestLat, requestLon float64, radiusMeters int) bool {
	return distanceMeters(centerLat, centerLon, requestLat, requestLon) <= float64(radiusMeters)
}

// distanceMeters returns the great-circle distance in meters between two
// coordinates using the haversine formula.
func distanceMeters(lat1, lon1, lat2, lon2 float64) float64 {
	phi1 := lat1 / 180.0 * math.Pi
	lambda1 := lon1 / 180.0 * math.Pi
	phi2 := lat2 / 180.0 * math.Pi
	lambda2 := lon2 / 180.0 * math.Pi

	deltaPhi := phi2 - phi1
	deltaLambda := lambda2 - lambda1

	a := math.Sin(deltaPhi/2)*math.Sin(deltaPhi/2) +
		math.Cos(phi1)*math.Cos(phi2)*math.Sin(deltaLambda/2)*math.Sin(deltaLambda/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusMeters * c
}
