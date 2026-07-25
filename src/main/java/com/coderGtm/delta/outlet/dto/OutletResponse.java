package com.coderGtm.delta.outlet.dto;

import java.math.BigDecimal;
import java.time.Instant;
import java.util.UUID;

/**
 * API representation of an outlet.
 */
public record OutletResponse(
	UUID id,
	String name,
	BigDecimal latitude,
	BigDecimal longitude,
	Integer radiusMeters,
	Instant createdAt,
	Instant updatedAt
) {
}
