package com.coderGtm.delta.report.controller;

import java.math.BigDecimal;
import java.time.Instant;
import java.util.UUID;

import org.springframework.format.annotation.DateTimeFormat;
import org.springframework.http.ContentDisposition;
import org.springframework.http.HttpHeaders;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.security.core.annotation.AuthenticationPrincipal;
import org.springframework.validation.annotation.Validated;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

import com.coderGtm.delta.common.web.ApiPaths;
import com.coderGtm.delta.report.dto.SalaryReportResponse;
import com.coderGtm.delta.report.service.SalaryReportService;
import com.coderGtm.delta.user.User;

import jakarta.validation.constraints.DecimalMin;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.RequiredArgsConstructor;

/**
 * Owner-facing report endpoints for outlet attendance and payroll summaries.
 */
@RestController
@RequestMapping(ApiPaths.OUTLETS)
@RequiredArgsConstructor
@Validated
public class ReportController {

	private final SalaryReportService salaryReportService;

	/**
	 * Calculates a salary report for one employee in the outlet over an exact
	 * timestamp range, grouping report rows by the requested timezone.
	 */
	@GetMapping("/{outletId}/reports/salary")
	public ResponseEntity<SalaryReportResponse> getSalaryReport(
		@AuthenticationPrincipal User currentUser,
		@PathVariable UUID outletId,
		@RequestParam @NotNull(message = "User ID is required") UUID userId,
		@RequestParam @DateTimeFormat(iso = DateTimeFormat.ISO.DATE_TIME) @NotNull(message = "Start time is required") Instant startTime,
		@RequestParam @DateTimeFormat(iso = DateTimeFormat.ISO.DATE_TIME) @NotNull(message = "End time is required") Instant endTime,
		@RequestParam @NotBlank(message = "Timezone is required") String timezone,
		@RequestParam @NotNull(message = "Hourly rate is required")
		@DecimalMin(value = "0.0", inclusive = false, message = "Hourly rate must be greater than zero") BigDecimal hourlyRate
	) {
		return ResponseEntity.ok(salaryReportService.calculateSalaryReport(
			currentUser.getId(),
			outletId,
			userId,
			startTime,
			endTime,
			timezone,
			hourlyRate
		));
	}

	/**
	 * Exports the salary report as an Excel workbook.
	 */
	@GetMapping("/{outletId}/reports/salary.xlsx")
	public ResponseEntity<byte[]> exportSalaryReport(
		@AuthenticationPrincipal User currentUser,
		@PathVariable UUID outletId,
		@RequestParam @NotNull(message = "User ID is required") UUID userId,
		@RequestParam @DateTimeFormat(iso = DateTimeFormat.ISO.DATE_TIME) @NotNull(message = "Start time is required") Instant startTime,
		@RequestParam @DateTimeFormat(iso = DateTimeFormat.ISO.DATE_TIME) @NotNull(message = "End time is required") Instant endTime,
		@RequestParam @NotBlank(message = "Timezone is required") String timezone,
		@RequestParam @NotNull(message = "Hourly rate is required")
		@DecimalMin(value = "0.0", inclusive = false, message = "Hourly rate must be greater than zero") BigDecimal hourlyRate
	) {
		byte[] workbook = salaryReportService.generateSalaryReportExcel(
			currentUser.getId(),
			outletId,
			userId,
			startTime,
			endTime,
			timezone,
			hourlyRate
		);

		String filename = "salary-report-%s-%s-to-%s.xlsx".formatted(
			userId,
			sanitizeForFilename(startTime.toString()),
			sanitizeForFilename(endTime.toString())
		);
		return ResponseEntity.ok()
			.contentType(MediaType.parseMediaType("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"))
			.header(HttpHeaders.CONTENT_DISPOSITION, ContentDisposition.attachment().filename(filename).build().toString())
			.body(workbook);
	}

	private String sanitizeForFilename(String value) {
		return value.replace(":", "-");
	}
}
