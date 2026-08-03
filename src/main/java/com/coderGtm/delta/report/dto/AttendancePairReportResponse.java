package com.coderGtm.delta.report.dto;

import java.math.BigDecimal;
import java.time.Instant;

/**
 * Completed clock-in/clock-out pair used in salary report calculations.
 */
public record AttendancePairReportResponse(
	Instant clockIn,
	Instant clockOut,
	BigDecimal hours
) {
}
