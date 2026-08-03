package com.coderGtm.delta.outlet.dto;

import jakarta.validation.constraints.NotNull;

/**
 * Request payload used by outlet owners to toggle attendance geofence
 * enforcement for an outlet.
 */
public record UpdateOutletGeofenceRequest(
	@NotNull(message = "Geofence enabled flag is required")
	Boolean geofenceEnabled
) {
}
