package com.coderGtm.delta.outlet.dto;

import java.math.BigDecimal;

import jakarta.validation.constraints.DecimalMax;
import jakarta.validation.constraints.DecimalMin;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Positive;
import jakarta.validation.constraints.Size;

/**
 * Request payload used to create a new outlet.
 */
public record CreateOutletRequest(
	@NotBlank(message = "Outlet name is required")
	@Size(max = 150, message = "Outlet name must be at most 150 characters")
	String name,

	@NotNull(message = "Latitude is required")
	@DecimalMin(value = "-90.0", message = "Latitude must be greater than or equal to -90")
	@DecimalMax(value = "90.0", message = "Latitude must be less than or equal to 90")
	BigDecimal latitude,

	@NotNull(message = "Longitude is required")
	@DecimalMin(value = "-180.0", message = "Longitude must be greater than or equal to -180")
	@DecimalMax(value = "180.0", message = "Longitude must be less than or equal to 180")
	BigDecimal longitude,

	@NotNull(message = "Radius in meters is required")
	@Positive(message = "Radius in meters must be greater than zero")
	Integer radiusMeters
) {
}
