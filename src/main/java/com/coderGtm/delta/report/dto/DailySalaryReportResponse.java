package com.coderGtm.delta.report.dto;

import java.math.BigDecimal;
import java.time.LocalDate;
import java.util.List;

/**
 * Salary-report row for a single calendar day in UTC.
 */
public record DailySalaryReportResponse(
	LocalDate date,
	List<AttendancePairReportResponse> attendancePairs,
	BigDecimal totalHours,
	BigDecimal hourlyRate,
	BigDecimal salary
) {
}
