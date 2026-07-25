package com.coderGtm.delta.attendance.dto;

import java.math.BigDecimal;
import java.time.Instant;
import java.util.UUID;

import com.coderGtm.delta.attendance.entity.AttendanceEntryType;

import jakarta.validation.constraints.DecimalMax;
import jakarta.validation.constraints.DecimalMin;
import jakarta.validation.constraints.NotNull;

/**
 * Request payload for owner-managed attendance creation on behalf of an
 * employee.
 */
public record ManageAttendanceEntryRequest(
	@NotNull(message = "User ID is required")
	UUID userId,

	@NotNull(message = "Attendance type is required")
	AttendanceEntryType type,

	@NotNull(message = "Entry time is required")
	Instant entryTime,

	@NotNull(message = "Latitude is required")
	@DecimalMin(value = "-90.0", message = "Latitude must be greater than or equal to -90")
	@DecimalMax(value = "90.0", message = "Latitude must be less than or equal to 90")
	BigDecimal latitude,

	@NotNull(message = "Longitude is required")
	@DecimalMin(value = "-180.0", message = "Longitude must be greater than or equal to -180")
	@DecimalMax(value = "180.0", message = "Longitude must be less than or equal to 180")
	BigDecimal longitude
) {
}
