package com.coderGtm.delta.common.util;

import java.math.BigDecimal;

/**
 * Utility methods for geographic distance calculations used by outlet
 * geofencing rules.
 */
public final class GeoUtils {

	private static final double EARTH_RADIUS_METERS = 6_371_000d;

	private GeoUtils() {
	}

	/**
	 * Returns whether the provided coordinates fall within the given radius from
	 * the configured outlet center.
	 */
	public static boolean isWithinRadiusMeters(
		BigDecimal centerLatitude,
		BigDecimal centerLongitude,
		BigDecimal requestLatitude,
		BigDecimal requestLongitude,
		int radiusMeters
	) {
		return distanceMeters(centerLatitude, centerLongitude, requestLatitude, requestLongitude) <= radiusMeters;
	}

	/**
	 * Calculates the great-circle distance in meters between two coordinates
	 * using the haversine formula.
	 */
	public static double distanceMeters(
		BigDecimal firstLatitude,
		BigDecimal firstLongitude,
		BigDecimal secondLatitude,
		BigDecimal secondLongitude
	) {
		double lat1 = Math.toRadians(firstLatitude.doubleValue());
		double lon1 = Math.toRadians(firstLongitude.doubleValue());
		double lat2 = Math.toRadians(secondLatitude.doubleValue());
		double lon2 = Math.toRadians(secondLongitude.doubleValue());

		double deltaLat = lat2 - lat1;
		double deltaLon = lon2 - lon1;

		double a = Math.pow(Math.sin(deltaLat / 2), 2)
			+ Math.cos(lat1) * Math.cos(lat2) * Math.pow(Math.sin(deltaLon / 2), 2);
		double c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
		return EARTH_RADIUS_METERS * c;
	}
}
