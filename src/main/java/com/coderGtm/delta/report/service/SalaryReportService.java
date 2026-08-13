package com.coderGtm.delta.report.service;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.math.BigDecimal;
import java.math.RoundingMode;
import java.time.DateTimeException;
import java.time.Duration;
import java.time.Instant;
import java.time.LocalDate;
import java.time.ZoneId;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;

import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.UUID;

import org.apache.poi.ss.usermodel.Cell;
import org.apache.poi.ss.usermodel.CellStyle;
import org.apache.poi.ss.usermodel.Font;
import org.apache.poi.ss.usermodel.Row;
import org.apache.poi.ss.usermodel.Sheet;
import org.apache.poi.ss.usermodel.Workbook;
import org.apache.poi.xssf.usermodel.XSSFWorkbook;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import com.coderGtm.delta.attendance.entity.AttendanceEntry;
import com.coderGtm.delta.attendance.entity.AttendanceEntryType;
import com.coderGtm.delta.attendance.repository.AttendanceEntryRepository;
import com.coderGtm.delta.common.audit.service.AuditService;
import com.coderGtm.delta.common.exception.BadRequestException;
import com.coderGtm.delta.common.exception.ForbiddenException;
import com.coderGtm.delta.common.exception.ResourceNotFoundException;
import com.coderGtm.delta.common.metrics.ApplicationMetrics;
import com.coderGtm.delta.outlet.entity.Outlet;
import com.coderGtm.delta.outlet.entity.OutletMembership;
import com.coderGtm.delta.outlet.entity.OutletMembershipStatus;
import com.coderGtm.delta.outlet.entity.OutletRole;
import com.coderGtm.delta.outlet.repository.OutletMembershipRepository;
import com.coderGtm.delta.report.dto.AttendancePairReportResponse;
import com.coderGtm.delta.report.dto.DailySalaryReportResponse;
import com.coderGtm.delta.report.dto.SalaryReportResponse;
import com.coderGtm.delta.user.User;

import lombok.RequiredArgsConstructor;

/**
 * Builds owner-facing salary reports from completed attendance clock-in/out
 * pairs.
 */
@Service
@RequiredArgsConstructor
public class SalaryReportService {

	private static final int MAX_REPORT_DAYS = 366;
	private static final DateTimeFormatter TIME_FORMATTER = DateTimeFormatter.ofPattern("HH:mm:ss");

	private final AttendanceEntryRepository attendanceEntryRepository;
	private final OutletMembershipRepository outletMembershipRepository;
	private final AuditService auditService;
	private final ApplicationMetrics applicationMetrics;

	/**
	 * Calculates salary data for one employee in one outlet over an exact instant
	 * range, grouping daily rows by the supplied timezone.
	 */
	@Transactional(readOnly = true)
	public SalaryReportResponse calculateSalaryReport(
		UUID currentUserId,
		UUID outletId,
		UUID employeeUserId,
		Instant startTime,
		Instant endTime,
		String timezone,
		BigDecimal hourlyRate
	) {
		ZoneId zoneId = validateRequest(startTime, endTime, timezone, hourlyRate);
		OutletMembership ownerMembership = assertAcceptedOwner(outletId, currentUserId);
		OutletMembership employeeMembership = outletMembershipRepository.findByOutlet_IdAndUser_Id(outletId, employeeUserId)
			.orElseThrow(() -> new ResourceNotFoundException("Outlet membership was not found for the requested employee"));

		List<AttendanceEntry> entries = attendanceEntryRepository
			.findAllByOutlet_IdAndUser_IdAndEntryTimeGreaterThanEqualAndEntryTimeLessThanOrderByEntryTimeAsc(
				outletId,
				employeeUserId,
				startTime,
				endTime
			);

		SalaryReportResponse report = buildReport(
			ownerMembership.getOutlet(),
			employeeMembership,
			startTime,
			endTime,
			zoneId,
			hourlyRate,
			entries
		);

		applicationMetrics.increment("report.salary.generated", "format", "json");
		auditService.record(
			currentUserId,
			"SALARY_REPORT_GENERATED",
			"OUTLET",
			outletId,
			Map.of(
				"employeeUserId", employeeUserId,
				"startTime", startTime.toString(),
				"endTime", endTime.toString(),
				"timezone", zoneId.getId(),
				"format", "json"
			)
		);

		return report;
	}

	/**
	 * Generates an Excel workbook for the salary report.
	 */
	@Transactional(readOnly = true)
	public byte[] generateSalaryReportExcel(
		UUID currentUserId,
		UUID outletId,
		UUID employeeUserId,
		Instant startTime,
		Instant endTime,
		String timezone,
		BigDecimal hourlyRate
	) {
		SalaryReportResponse report = calculateSalaryReport(
			currentUserId,
			outletId,
			employeeUserId,
			startTime,
			endTime,
			timezone,
			hourlyRate
		);
		applicationMetrics.increment("report.salary.generated", "format", "xlsx");
		auditService.record(
			currentUserId,
			"SALARY_REPORT_EXCEL_GENERATED",
			"OUTLET",
			outletId,
			Map.of(
				"employeeUserId", employeeUserId,
				"startTime", startTime.toString(),
				"endTime", endTime.toString(),
				"timezone", report.timezone(),
				"format", "xlsx"
			)
		);

		try (Workbook workbook = new XSSFWorkbook(); ByteArrayOutputStream outputStream = new ByteArrayOutputStream()) {
			writeWorkbook(workbook, report);
			workbook.write(outputStream);
			return outputStream.toByteArray();
		} catch (IOException ex) {
			throw new IllegalStateException("Failed to generate salary report Excel workbook", ex);
		}
	}

	private SalaryReportResponse buildReport(
		Outlet outlet,
		OutletMembership employeeMembership,
		Instant startTime,
		Instant endTime,
		ZoneId zoneId,
		BigDecimal hourlyRate,
		List<AttendanceEntry> entries
	) {
		User employee = employeeMembership.getUser();
		LocalDate startDate = startTime.atZone(zoneId).toLocalDate();
		LocalDate endDate = endTime.minusNanos(1).atZone(zoneId).toLocalDate();
		Map<LocalDate, List<AttendanceEntry>> entriesByDate = groupByDate(entries, zoneId);
		List<DailySalaryReportResponse> days = new ArrayList<>();
		BigDecimal totalHours = BigDecimal.ZERO;
		BigDecimal totalSalary = BigDecimal.ZERO;

		for (LocalDate date = startDate; !date.isAfter(endDate); date = date.plusDays(1)) {
			List<AttendancePairReportResponse> pairs = completedPairs(entriesByDate.getOrDefault(date, List.of()));
			BigDecimal dayHours = pairs.stream()
				.map(AttendancePairReportResponse::hours)
				.reduce(BigDecimal.ZERO, BigDecimal::add)
				.setScale(2, RoundingMode.HALF_UP);
			BigDecimal daySalary = dayHours.multiply(hourlyRate).setScale(2, RoundingMode.HALF_UP);

			days.add(new DailySalaryReportResponse(date, pairs, dayHours, hourlyRate, daySalary));
			totalHours = totalHours.add(dayHours);
			totalSalary = totalSalary.add(daySalary);
		}

		return new SalaryReportResponse(
			outlet.getId(),
			outlet.getName(),
			employee.getId(),
			employee.getName(),
			employee.getEmail(),
			displayName(employeeMembership),
			startTime,
			endTime,
			zoneId.getId(),
			hourlyRate,
			totalHours.setScale(2, RoundingMode.HALF_UP),
			totalSalary.setScale(2, RoundingMode.HALF_UP),
			days
		);
	}

	private Map<LocalDate, List<AttendanceEntry>> groupByDate(List<AttendanceEntry> entries, ZoneId zoneId) {
		Map<LocalDate, List<AttendanceEntry>> entriesByDate = new HashMap<>();
		for (AttendanceEntry entry : entries) {
			LocalDate date = entry.getEntryTime().atZone(zoneId).toLocalDate();
			entriesByDate.computeIfAbsent(date, ignored -> new ArrayList<>()).add(entry);
		}
		return entriesByDate;
	}

	private List<AttendancePairReportResponse> completedPairs(List<AttendanceEntry> entries) {
		List<AttendancePairReportResponse> pairs = new ArrayList<>();
		AttendanceEntry pendingClockIn = null;

		for (AttendanceEntry entry : entries) {
			if (entry.getType() == AttendanceEntryType.CLOCK_IN) {
				pendingClockIn = entry;
				continue;
			}

			if (entry.getType() == AttendanceEntryType.CLOCK_OUT && pendingClockIn != null
				&& entry.getEntryTime().isAfter(pendingClockIn.getEntryTime())) {
				pairs.add(new AttendancePairReportResponse(
					pendingClockIn.getEntryTime(),
					entry.getEntryTime(),
					hoursBetween(pendingClockIn.getEntryTime(), entry.getEntryTime())
				));
				pendingClockIn = null;
			}
		}

		return pairs;
	}

	private BigDecimal hoursBetween(Instant clockIn, Instant clockOut) {
		long seconds = Duration.between(clockIn, clockOut).getSeconds();
		return BigDecimal.valueOf(seconds)
			.divide(BigDecimal.valueOf(3600), 2, RoundingMode.HALF_UP);
	}

	private String displayName(OutletMembership membership) {
		return membership.getDisplayName() != null ? membership.getDisplayName() : membership.getUser().getName();
	}

	private void writeWorkbook(Workbook workbook, SalaryReportResponse report) {
		ZoneId zoneId = ZoneId.of(report.timezone());
		Sheet sheet = workbook.createSheet("Salary Report");
		int maxPairs = report.days().stream()
			.mapToInt(day -> day.attendancePairs().size())
			.max()
			.orElse(0);
		CellStyle headerStyle = headerStyle(workbook);
		CellStyle totalStyle = totalStyle(workbook);

		int rowIndex = 0;
		Row titleRow = sheet.createRow(rowIndex++);
		createCell(titleRow, 0, "Salary Report", headerStyle);

		Row metadataRow = sheet.createRow(rowIndex++);
		createCell(metadataRow, 0, "Outlet", headerStyle);
		createCell(metadataRow, 1, report.outletName(), null);
		createCell(metadataRow, 2, "Employee", headerStyle);
		createCell(metadataRow, 3, report.displayName() + " <" + report.userEmail() + ">", null);
		createCell(metadataRow, 4, "Period", headerStyle);
		createCell(metadataRow, 5, report.startTime().atZone(zoneId) + " to " + report.endTime().atZone(zoneId), null);
		createCell(metadataRow, 6, "Timezone", headerStyle);
		createCell(metadataRow, 7, report.timezone(), null);

		rowIndex++;
		Row headerRow = sheet.createRow(rowIndex++);
		int column = 0;
		createCell(headerRow, column++, "Date", headerStyle);
		for (int index = 1; index <= maxPairs; index++) {
			createCell(headerRow, column++, "Clock In " + index, headerStyle);
			createCell(headerRow, column++, "Clock Out " + index, headerStyle);
		}
		createCell(headerRow, column++, "Total Hours", headerStyle);
		createCell(headerRow, column++, "Hourly Rate", headerStyle);
		createCell(headerRow, column, "Salary", headerStyle);

		for (DailySalaryReportResponse day : report.days()) {
			Row row = sheet.createRow(rowIndex++);
			column = 0;
			createCell(row, column++, day.date().toString(), null);
			for (AttendancePairReportResponse pair : day.attendancePairs()) {
				createCell(row, column++, TIME_FORMATTER.withZone(zoneId).format(pair.clockIn()), null);
				createCell(row, column++, TIME_FORMATTER.withZone(zoneId).format(pair.clockOut()), null);
			}
			while (column < 1 + (maxPairs * 2)) {
				createCell(row, column++, "", null);
			}
			createCell(row, column++, day.totalHours(), null);
			createCell(row, column++, day.hourlyRate(), null);
			createCell(row, column, day.salary(), null);
		}

		Row totalRow = sheet.createRow(rowIndex);
		column = 0;
		createCell(totalRow, column++, "TOTAL", totalStyle);
		while (column < 1 + (maxPairs * 2)) {
			createCell(totalRow, column++, "", totalStyle);
		}
		createCell(totalRow, column++, report.totalHours(), totalStyle);
		createCell(totalRow, column++, report.hourlyRate(), totalStyle);
		createCell(totalRow, column, report.totalSalary(), totalStyle);

		for (int index = 0; index <= 5 + (maxPairs * 2); index++) {
			sheet.autoSizeColumn(index);
		}
	}

	private CellStyle headerStyle(Workbook workbook) {
		CellStyle style = workbook.createCellStyle();
		Font font = workbook.createFont();
		font.setBold(true);
		style.setFont(font);
		return style;
	}

	private CellStyle totalStyle(Workbook workbook) {
		return headerStyle(workbook);
	}

	private void createCell(Row row, int column, String value, CellStyle style) {
		Cell cell = row.createCell(column);
		cell.setCellValue(sanitizeCellValue(value));
		if (style != null) {
			cell.setCellStyle(style);
		}
	}

	/**
	 * Defends against spreadsheet formula injection (CWE-1236). Leading cells
	 * such as {@code =}, {@code +}, {@code -}, {@code @} or control characters
	 * are interpreted as formulas by spreadsheet applications. Prefixing such
	 * values with a single quote forces them to be treated as literal text while
	 * presenting the original value on output.
	 */
	private String sanitizeCellValue(String value) {
		if (value == null || value.isEmpty()) {
			return value;
		}

		char first = value.charAt(0);
		if (first == '=' || first == '+' || first == '-' || first == '@'
			|| first <= 0x20 || first == 0x7F) {
			return "'" + value;
		}
		return value;
	}

	private void createCell(Row row, int column, BigDecimal value, CellStyle style) {
		Cell cell = row.createCell(column);
		cell.setCellValue(value.doubleValue());
		if (style != null) {
			cell.setCellStyle(style);
		}
	}

	private ZoneId validateRequest(Instant startTime, Instant endTime, String timezone, BigDecimal hourlyRate) {
		if (startTime == null || endTime == null) {
			throw new BadRequestException("Start time and end time are required");
		}
		if (!endTime.isAfter(startTime)) {
			throw new BadRequestException("End time must be after start time");
		}

		ZoneId zoneId;
		try {
			zoneId = ZoneId.of(timezone);
		} catch (DateTimeException ex) {
			throw new BadRequestException("Timezone must be a valid IANA timezone");
		}

		LocalDate startDate = startTime.atZone(zoneId).toLocalDate();
		LocalDate endDate = endTime.minusNanos(1).atZone(zoneId).toLocalDate();
		if (startDate.plusDays(MAX_REPORT_DAYS - 1).isBefore(endDate)) {
			throw new BadRequestException("Salary reports can cover at most 366 local days");
		}
		if (hourlyRate == null || hourlyRate.compareTo(BigDecimal.ZERO) <= 0) {
			throw new BadRequestException("Hourly rate must be greater than zero");
		}

		return zoneId;
	}

	private OutletMembership assertAcceptedOwner(UUID outletId, UUID currentUserId) {
		OutletMembership membership = outletMembershipRepository.findByOutlet_IdAndUser_IdAndRemovedAtIsNull(outletId, currentUserId)
			.orElseThrow(() -> new ResourceNotFoundException("Outlet membership was not found for the current user"));

		if (membership.getStatus() != OutletMembershipStatus.ACCEPTED) {
			throw new ForbiddenException("You must accept the outlet invitation before accessing this outlet");
		}

		if (membership.getRole() != OutletRole.OWNER) {
			throw new ForbiddenException("Only outlet owners can perform this action");
		}

		return membership;
	}
}
