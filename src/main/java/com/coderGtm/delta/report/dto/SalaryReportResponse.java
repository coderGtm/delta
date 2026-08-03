package com.coderGtm.delta.report.dto;

import java.math.BigDecimal;
import java.time.Instant;
import java.util.List;
import java.util.UUID;

/**
 * Owner-facing salary report for one employee in one outlet over a selected
 * timestamp range and timezone.
 */
public record SalaryReportResponse(
	UUID outletId,
	String outletName,
	UUID userId,
	String userName,
	String userEmail,
	Instant startTime,
	Instant endTime,
	String timezone,
	BigDecimal hourlyRate,
	BigDecimal totalHours,
	BigDecimal totalSalary,
	List<DailySalaryReportResponse> days
) {
}
