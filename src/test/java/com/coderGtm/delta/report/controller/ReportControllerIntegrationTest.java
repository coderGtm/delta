package com.coderGtm.delta.report.controller;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.header;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import java.math.BigDecimal;
import java.time.Instant;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.web.context.WebApplicationContext;

import com.coderGtm.delta.TestApplication;
import com.coderGtm.delta.TestFirebaseConfiguration;
import com.coderGtm.delta.attendance.entity.AttendanceEntry;
import com.coderGtm.delta.attendance.entity.AttendanceEntryType;
import com.coderGtm.delta.attendance.repository.AttendanceEntryRepository;
import com.coderGtm.delta.auth.service.JwtService;
import com.coderGtm.delta.outlet.entity.Outlet;
import com.coderGtm.delta.outlet.entity.OutletMembership;
import com.coderGtm.delta.outlet.entity.OutletMembershipStatus;
import com.coderGtm.delta.outlet.entity.OutletRole;
import com.coderGtm.delta.outlet.repository.OutletMembershipRepository;
import com.coderGtm.delta.outlet.repository.OutletRepository;
import com.coderGtm.delta.user.User;
import com.coderGtm.delta.user.UserRepository;

import jakarta.servlet.Filter;

@SpringBootTest(classes = { TestApplication.class, TestFirebaseConfiguration.class })
@Transactional
class ReportControllerIntegrationTest {

	@Autowired
	private WebApplicationContext context;

	@Autowired
	private Filter springSecurityFilterChain;

	@Autowired
	private JwtService jwtService;

	@Autowired
	private UserRepository userRepository;

	@Autowired
	private OutletRepository outletRepository;

	@Autowired
	private OutletMembershipRepository outletMembershipRepository;

	@Autowired
	private AttendanceEntryRepository attendanceEntryRepository;

	private MockMvc mockMvc;

	@BeforeEach
	void setUp() {
		mockMvc = MockMvcBuilders.webAppContextSetup(context)
			.addFilters(springSecurityFilterChain)
			.build();
	}

	@Test
	void getSalaryReportReturnsCalculatedTotalsForOwner() throws Exception {
		Fixture fixture = fixture();

		mockMvc.perform(get("/api/v1/outlets/{outletId}/reports/salary", fixture.outlet().getId())
				.header("Authorization", "Bearer " + jwtService.generateAccessToken(fixture.owner()))
				.param("userId", fixture.employee().getId().toString())
				.param("startTime", "2024-01-01T00:00:00Z")
				.param("endTime", "2024-01-02T00:00:00Z")
				.param("timezone", "UTC")
				.param("hourlyRate", "100.00"))
			.andExpect(status().isOk())
			.andExpect(jsonPath("$.userId").value(fixture.employee().getId().toString()))
			.andExpect(jsonPath("$.displayName").value("Test User employee"))
			.andExpect(jsonPath("$.startTime").value("2024-01-01T00:00:00Z"))
			.andExpect(jsonPath("$.endTime").value("2024-01-02T00:00:00Z"))
			.andExpect(jsonPath("$.timezone").value("UTC"))
			.andExpect(jsonPath("$.days[0].totalHours").value(8.5))
			.andExpect(jsonPath("$.days[0].salary").value(850.0))
			.andExpect(jsonPath("$.totalSalary").value(850.0));
	}

	@Test
	void exportSalaryReportReturnsExcelAttachmentForOwner() throws Exception {
		Fixture fixture = fixture();

		mockMvc.perform(get("/api/v1/outlets/{outletId}/reports/salary.xlsx", fixture.outlet().getId())
				.header("Authorization", "Bearer " + jwtService.generateAccessToken(fixture.owner()))
				.param("userId", fixture.employee().getId().toString())
				.param("startTime", "2024-01-01T00:00:00Z")
				.param("endTime", "2024-01-02T00:00:00Z")
				.param("timezone", "UTC")
				.param("hourlyRate", "100.00"))
			.andExpect(status().isOk())
			.andExpect(header().string("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"))
			.andExpect(header().string("Content-Disposition", org.hamcrest.Matchers.containsString("salary-report-")));
	}

	private Fixture fixture() {
		User owner = persistUser("owner");
		User employee = persistUser("employee");
		Outlet outlet = persistOutlet();
		persistMembership(outlet, owner, OutletRole.OWNER);
		persistMembership(outlet, employee, OutletRole.EMPLOYEE);
		persistAttendance(outlet, employee, AttendanceEntryType.CLOCK_IN, "2024-01-01T09:00:00Z");
		persistAttendance(outlet, employee, AttendanceEntryType.CLOCK_OUT, "2024-01-01T17:30:00Z");
		return new Fixture(owner, employee, outlet);
	}

	private User persistUser(String authUidPrefix) {
		User user = new User();
		user.setAuthUid(authUidPrefix + "-uid");
		user.setName("Test User " + authUidPrefix);
		user.setEmail(authUidPrefix + "@example.com");
		user.setPhone("+911234567890");
		return userRepository.saveAndFlush(user);
	}

	private Outlet persistOutlet() {
		Outlet outlet = new Outlet();
		outlet.setName("Outlet A");
		outlet.setLatitude(new BigDecimal("12.9715987"));
		outlet.setLongitude(new BigDecimal("77.5945627"));
		outlet.setRadiusMeters(150);
		outlet.setGeofenceEnabled(false);
		return outletRepository.saveAndFlush(outlet);
	}

	private void persistMembership(Outlet outlet, User user, OutletRole role) {
		OutletMembership membership = new OutletMembership();
		membership.setOutlet(outlet);
		membership.setUser(user);
		membership.setDisplayName(user.getName());
		membership.setRole(role);
		membership.setStatus(OutletMembershipStatus.ACCEPTED);
		outletMembershipRepository.saveAndFlush(membership);
	}

	private void persistAttendance(Outlet outlet, User user, AttendanceEntryType type, String entryTime) {
		AttendanceEntry entry = new AttendanceEntry();
		entry.setOutlet(outlet);
		entry.setUser(user);
		entry.setType(type);
		entry.setEntryTime(Instant.parse(entryTime));
		entry.setLatitude(new BigDecimal("12.9715987"));
		entry.setLongitude(new BigDecimal("77.5945627"));
		attendanceEntryRepository.saveAndFlush(entry);
	}

	private record Fixture(User owner, User employee, Outlet outlet) {
	}
}
