package com.coderGtm.delta.report.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.Mockito.when;

import java.io.ByteArrayInputStream;
import java.math.BigDecimal;
import java.time.Instant;
import java.util.List;
import java.util.Optional;
import java.util.UUID;

import org.apache.poi.ss.usermodel.Workbook;
import org.apache.poi.xssf.usermodel.XSSFWorkbook;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import com.coderGtm.delta.attendance.entity.AttendanceEntry;
import com.coderGtm.delta.attendance.entity.AttendanceEntryType;
import com.coderGtm.delta.attendance.repository.AttendanceEntryRepository;
import com.coderGtm.delta.common.audit.service.AuditService;
import com.coderGtm.delta.common.exception.ForbiddenException;
import com.coderGtm.delta.common.metrics.ApplicationMetrics;
import com.coderGtm.delta.outlet.entity.Outlet;
import com.coderGtm.delta.outlet.entity.OutletMembership;
import com.coderGtm.delta.outlet.entity.OutletMembershipStatus;
import com.coderGtm.delta.outlet.entity.OutletRole;
import com.coderGtm.delta.outlet.repository.OutletMembershipRepository;
import com.coderGtm.delta.report.dto.SalaryReportResponse;
import com.coderGtm.delta.user.User;

@ExtendWith(MockitoExtension.class)
class SalaryReportServiceTest {

	@Mock
	private AttendanceEntryRepository attendanceEntryRepository;

	@Mock
	private OutletMembershipRepository outletMembershipRepository;

	@Mock
	private AuditService auditService;

	@Mock
	private ApplicationMetrics applicationMetrics;

	private SalaryReportService salaryReportService;

	@BeforeEach
	void setUp() {
		salaryReportService = new SalaryReportService(
			attendanceEntryRepository,
			outletMembershipRepository,
			auditService,
			applicationMetrics
		);
	}

	@Test
	void calculateSalaryReportPairsCompletedEntriesPerSelectedTimezoneDay() {
		UUID outletId = UUID.randomUUID();
		UUID ownerId = UUID.randomUUID();
		UUID employeeId = UUID.randomUUID();
		Outlet outlet = outlet(outletId, "Outlet A");
		User owner = user(ownerId, "owner@example.com", "Owner");
		User employee = user(employeeId, "employee@example.com", "Employee");
		OutletMembership ownerMembership = membership(outlet, owner, OutletRole.OWNER, OutletMembershipStatus.ACCEPTED);
		OutletMembership employeeMembership = membership(outlet, employee, OutletRole.EMPLOYEE, OutletMembershipStatus.ACCEPTED);
		Instant startTime = Instant.parse("2024-01-01T00:00:00Z");
		Instant endTime = Instant.parse("2024-01-04T00:00:00Z");

		when(outletMembershipRepository.findByOutlet_IdAndUser_IdAndRemovedAtIsNull(outletId, ownerId))
			.thenReturn(Optional.of(ownerMembership));
		when(outletMembershipRepository.findByOutlet_IdAndUser_Id(outletId, employeeId))
			.thenReturn(Optional.of(employeeMembership));
		when(attendanceEntryRepository.findAllByOutlet_IdAndUser_IdAndEntryTimeGreaterThanEqualAndEntryTimeLessThanOrderByEntryTimeAsc(
			outletId,
			employeeId,
			startTime,
			endTime
		)).thenReturn(List.of(
			attendance(outlet, employee, AttendanceEntryType.CLOCK_IN, "2024-01-01T09:00:00Z"),
			attendance(outlet, employee, AttendanceEntryType.CLOCK_OUT, "2024-01-01T12:00:00Z"),
			attendance(outlet, employee, AttendanceEntryType.CLOCK_IN, "2024-01-01T13:00:00Z"),
			attendance(outlet, employee, AttendanceEntryType.CLOCK_OUT, "2024-01-01T17:00:00Z"),
			attendance(outlet, employee, AttendanceEntryType.CLOCK_IN, "2024-01-02T10:00:00Z"),
			attendance(outlet, employee, AttendanceEntryType.CLOCK_OUT, "2024-01-02T14:00:00Z"),
			attendance(outlet, employee, AttendanceEntryType.CLOCK_IN, "2024-01-03T09:00:00Z")
		));

		SalaryReportResponse response = salaryReportService.calculateSalaryReport(
			ownerId,
			outletId,
			employeeId,
			startTime,
			endTime,
			"Asia/Kolkata",
			new BigDecimal("100.00")
		);

		assertThat(response.timezone()).isEqualTo("Asia/Kolkata");
		assertThat(response.startTime()).isEqualTo(startTime);
		assertThat(response.endTime()).isEqualTo(endTime);
		assertThat(response.displayName()).isEqualTo("Employee");
		assertThat(response.days()).hasSize(4);
		assertThat(response.days().get(0).attendancePairs()).hasSize(2);
		assertThat(response.days().get(0).totalHours()).isEqualByComparingTo("7.00");
		assertThat(response.days().get(0).salary()).isEqualByComparingTo("700.00");
		assertThat(response.days().get(1).totalHours()).isEqualByComparingTo("4.00");
		assertThat(response.totalHours()).isEqualByComparingTo("11.00");
		assertThat(response.totalSalary()).isEqualByComparingTo("1100.00");
	}

	@Test
	void calculateSalaryReportUsesTimezoneWhenGroupingRows() {
		UUID outletId = UUID.randomUUID();
		UUID ownerId = UUID.randomUUID();
		UUID employeeId = UUID.randomUUID();
		Outlet outlet = outlet(outletId, "Outlet A");
		User owner = user(ownerId, "owner@example.com", "Owner");
		User employee = user(employeeId, "employee@example.com", "Employee");
		OutletMembership ownerMembership = membership(outlet, owner, OutletRole.OWNER, OutletMembershipStatus.ACCEPTED);
		OutletMembership employeeMembership = membership(outlet, employee, OutletRole.EMPLOYEE, OutletMembershipStatus.ACCEPTED);
		Instant startTime = Instant.parse("2024-01-01T00:00:00Z");
		Instant endTime = Instant.parse("2024-01-02T00:00:00Z");

		when(outletMembershipRepository.findByOutlet_IdAndUser_IdAndRemovedAtIsNull(outletId, ownerId))
			.thenReturn(Optional.of(ownerMembership));
		when(outletMembershipRepository.findByOutlet_IdAndUser_Id(outletId, employeeId))
			.thenReturn(Optional.of(employeeMembership));
		when(attendanceEntryRepository.findAllByOutlet_IdAndUser_IdAndEntryTimeGreaterThanEqualAndEntryTimeLessThanOrderByEntryTimeAsc(
			outletId,
			employeeId,
			startTime,
			endTime
		)).thenReturn(List.of(
			attendance(outlet, employee, AttendanceEntryType.CLOCK_IN, "2024-01-01T19:00:00Z"),
			attendance(outlet, employee, AttendanceEntryType.CLOCK_OUT, "2024-01-01T21:00:00Z")
		));

		SalaryReportResponse response = salaryReportService.calculateSalaryReport(
			ownerId,
			outletId,
			employeeId,
			startTime,
			endTime,
			"Asia/Kolkata",
			new BigDecimal("50.00")
		);

		assertThat(response.days()).hasSize(2);
		assertThat(response.days().get(0).date()).isEqualTo(java.time.LocalDate.parse("2024-01-01"));
		assertThat(response.days().get(0).totalHours()).isEqualByComparingTo("0.00");
		assertThat(response.days().get(1).date()).isEqualTo(java.time.LocalDate.parse("2024-01-02"));
		assertThat(response.days().get(1).totalHours()).isEqualByComparingTo("2.00");
	}

	@Test
	void generateSalaryReportExcelIncludesDynamicPairColumnsAndTimezoneAdjustedTimes() throws Exception {
		UUID outletId = UUID.randomUUID();
		UUID ownerId = UUID.randomUUID();
		UUID employeeId = UUID.randomUUID();
		Outlet outlet = outlet(outletId, "Outlet A");
		User owner = user(ownerId, "owner@example.com", "Owner");
		User employee = user(employeeId, "employee@example.com", "Employee");
		OutletMembership ownerMembership = membership(outlet, owner, OutletRole.OWNER, OutletMembershipStatus.ACCEPTED);
		OutletMembership employeeMembership = membership(outlet, employee, OutletRole.EMPLOYEE, OutletMembershipStatus.ACCEPTED);
		Instant startTime = Instant.parse("2024-01-01T00:00:00Z");
		Instant endTime = Instant.parse("2024-01-02T00:00:00Z");

		when(outletMembershipRepository.findByOutlet_IdAndUser_IdAndRemovedAtIsNull(outletId, ownerId))
			.thenReturn(Optional.of(ownerMembership));
		when(outletMembershipRepository.findByOutlet_IdAndUser_Id(outletId, employeeId))
			.thenReturn(Optional.of(employeeMembership));
		when(attendanceEntryRepository.findAllByOutlet_IdAndUser_IdAndEntryTimeGreaterThanEqualAndEntryTimeLessThanOrderByEntryTimeAsc(
			outletId,
			employeeId,
			startTime,
			endTime
		)).thenReturn(List.of(
			attendance(outlet, employee, AttendanceEntryType.CLOCK_IN, "2024-01-01T09:00:00Z"),
			attendance(outlet, employee, AttendanceEntryType.CLOCK_OUT, "2024-01-01T17:30:00Z")
		));

		byte[] workbookBytes = salaryReportService.generateSalaryReportExcel(
			ownerId,
			outletId,
			employeeId,
			startTime,
			endTime,
			"Asia/Kolkata",
			new BigDecimal("120.00")
		);

		try (Workbook workbook = new XSSFWorkbook(new ByteArrayInputStream(workbookBytes))) {
			assertThat(workbook.getSheet("Salary Report")).isNotNull();
			assertThat(workbook.getSheetAt(0).getRow(1).getCell(3).getStringCellValue()).isEqualTo("Employee <employee@example.com>");
			assertThat(workbook.getSheetAt(0).getRow(1).getCell(7).getStringCellValue()).isEqualTo("Asia/Kolkata");
			assertThat(workbook.getSheetAt(0).getRow(3).getCell(0).getStringCellValue()).isEqualTo("Date");
			assertThat(workbook.getSheetAt(0).getRow(3).getCell(1).getStringCellValue()).isEqualTo("Clock In 1");
			assertThat(workbook.getSheetAt(0).getRow(4).getCell(1).getStringCellValue()).isEqualTo("14:30:00");
			assertThat(workbook.getSheetAt(0).getRow(4).getCell(3).getNumericCellValue()).isEqualTo(8.50d);
			assertThat(workbook.getSheetAt(0).getRow(6).getCell(5).getNumericCellValue()).isEqualTo(1020.00d);
		}
	}

	@Test
	void calculateSalaryReportRejectsNonOwner() {
		UUID outletId = UUID.randomUUID();
		UUID employeeId = UUID.randomUUID();
		Outlet outlet = outlet(outletId, "Outlet A");
		User employee = user(employeeId, "employee@example.com", "Employee");
		OutletMembership employeeMembership = membership(outlet, employee, OutletRole.EMPLOYEE, OutletMembershipStatus.ACCEPTED);

		when(outletMembershipRepository.findByOutlet_IdAndUser_IdAndRemovedAtIsNull(outletId, employeeId))
			.thenReturn(Optional.of(employeeMembership));

		assertThatThrownBy(() -> salaryReportService.calculateSalaryReport(
			employeeId,
			outletId,
			employeeId,
			Instant.parse("2024-01-01T00:00:00Z"),
			Instant.parse("2024-02-01T00:00:00Z"),
			"UTC",
			new BigDecimal("100.00")
		))
			.isInstanceOf(ForbiddenException.class)
			.hasMessage("Only outlet owners can perform this action");
	}

	private User user(UUID id, String email, String name) {
		User user = new User();
		user.setId(id);
		user.setEmail(email);
		user.setName(name);
		return user;
	}

	private Outlet outlet(UUID id, String name) {
		Outlet outlet = new Outlet();
		outlet.setId(id);
		outlet.setName(name);
		outlet.setLatitude(new BigDecimal("12.9715987"));
		outlet.setLongitude(new BigDecimal("77.5945627"));
		outlet.setRadiusMeters(150);
		return outlet;
	}

	private OutletMembership membership(Outlet outlet, User user, OutletRole role, OutletMembershipStatus status) {
		OutletMembership membership = new OutletMembership();
		membership.setId(UUID.randomUUID());
		membership.setOutlet(outlet);
		membership.setUser(user);
		membership.setDisplayName(user.getName());
		membership.setRole(role);
		membership.setStatus(status);
		return membership;
	}

	private AttendanceEntry attendance(Outlet outlet, User user, AttendanceEntryType type, String entryTime) {
		AttendanceEntry entry = new AttendanceEntry();
		entry.setId(UUID.randomUUID());
		entry.setOutlet(outlet);
		entry.setUser(user);
		entry.setType(type);
		entry.setEntryTime(Instant.parse(entryTime));
		entry.setLatitude(new BigDecimal("12.9715987"));
		entry.setLongitude(new BigDecimal("77.5945627"));
		return entry;
	}
}
