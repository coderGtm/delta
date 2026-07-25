package com.coderGtm.delta.common.dto;

import java.time.Instant;

/**
 * Standard JSON error payload returned by the API.
 */
public record ErrorResponse(
	String code,
	String message,
	Instant timestamp
) {
}
