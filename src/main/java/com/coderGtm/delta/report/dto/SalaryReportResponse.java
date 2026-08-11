package com.coderGtm.delta.report.dto;

import java.math.BigDecimal;
import java.time.Instant;
import java.util.List;
import java.util.UUID;

/**
 * Owner-facing salary report for one employee in one outlet over a selected
 * timestamp range and timezone.
 *
 * <p>{@code displayName} is the outlet-scoped owner-controlled name to render
 * for the employee; it falls back to the user's account name when not set.</p>
 */
public record SalaryReportResponse(
	UUID outletId,
	String outletName,
	UUID userId,
	String userName,
	String userEmail,
	String displayName,
	Instant startTime,
	Instant endTime,
	String timezone,
	BigDecimal hourlyRate,
	BigDecimal totalHours,
	BigDecimal totalSalary,
	List<DailySalaryReportResponse> days
) {
}
