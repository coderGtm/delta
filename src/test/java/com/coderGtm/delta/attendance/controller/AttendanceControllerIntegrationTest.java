package com.coderGtm.delta.attendance.controller;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import java.math.BigDecimal;
import java.time.Instant;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
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
class AttendanceControllerIntegrationTest {

	@Autowired
	private WebApplicationContext context;

	@Autowired
	private Filter springSecurityFilterChain;

	@Autowired
	private UserRepository userRepository;

	@Autowired
	private OutletRepository outletRepository;

	@Autowired
	private OutletMembershipRepository outletMembershipRepository;

	@Autowired
	private AttendanceEntryRepository attendanceEntryRepository;

	@Autowired
	private JwtService jwtService;

	private MockMvc mockMvc;

	@BeforeEach
	void setUp() {
		mockMvc = MockMvcBuilders.webAppContextSetup(context)
			.addFilters(springSecurityFilterChain)
			.build();
	}

	@Test
	void createOwnAttendanceRejectsOutsideGeofenceWhenEnabled() throws Exception {
		User employee = persistUser("employee");
		Outlet outlet = persistOutlet("Outlet A", true);
		persistMembership(outlet, employee, OutletRole.EMPLOYEE, OutletMembershipStatus.ACCEPTED);

		mockMvc.perform(post("/api/v1/outlets/{outletId}/attendance", outlet.getId())
				.header("Authorization", "Bearer " + jwtService.generateAccessToken(employee))
				.contentType(MediaType.APPLICATION_JSON)
				.content("""
					{
					  \"type\": \"CLOCK_IN\",
					  \"latitude\": 13.0352000,
					  \"longitude\": 77.5970000
					}
					"""))
			.andExpect(status().isForbidden())
			.andExpect(jsonPath("$.code").value("FORBIDDEN"))
			.andExpect(jsonPath("$.message").value("Attendance location is outside the outlet geofence"));
	}

	@Test
	void createOwnAttendanceAllowsOutsideGeofenceWhenDisabled() throws Exception {
		User employee = persistUser("employee-2");
		Outlet outlet = persistOutlet("Outlet B", false);
		persistMembership(outlet, employee, OutletRole.EMPLOYEE, OutletMembershipStatus.ACCEPTED);

		mockMvc.perform(post("/api/v1/outlets/{outletId}/attendance", outlet.getId())
				.header("Authorization", "Bearer " + jwtService.generateAccessToken(employee))
				.contentType(MediaType.APPLICATION_JSON)
				.content("""
					{
					  \"type\": \"CLOCK_IN\",
					  \"latitude\": 13.0352000,
					  \"longitude\": 77.5970000
					}
					"""))
			.andExpect(status().isCreated())
			.andExpect(jsonPath("$.userId").value(employee.getId().toString()))
			.andExpect(jsonPath("$.type").value("CLOCK_IN"));

		assertThat(attendanceEntryRepository.findAll()).hasSize(1);
	}

	@Test
	void getAttendanceEntriesReturnsPaginatedResponseForOwner() throws Exception {
		User owner = persistUser("owner");
		User employee = persistUser("employee-3");
		Outlet outlet = persistOutlet("Outlet C", false);
		persistMembership(outlet, owner, OutletRole.OWNER, OutletMembershipStatus.ACCEPTED);
		persistMembership(outlet, employee, OutletRole.EMPLOYEE, OutletMembershipStatus.ACCEPTED);
		persistAttendance(outlet, employee, AttendanceEntryType.CLOCK_IN, Instant.parse("2024-01-01T09:00:00Z"));
		persistAttendance(outlet, employee, AttendanceEntryType.CLOCK_OUT, Instant.parse("2024-01-01T18:00:00Z"));

		mockMvc.perform(get("/api/v1/outlets/{outletId}/attendance", outlet.getId())
				.header("Authorization", "Bearer " + jwtService.generateAccessToken(owner))
				.param("page", "0")
				.param("size", "1"))
			.andExpect(status().isOk())
			.andExpect(jsonPath("$.content.length()").value(1))
			.andExpect(jsonPath("$.totalElements").value(2))
			.andExpect(jsonPath("$.totalPages").value(2))
			.andExpect(jsonPath("$.content[0].userId").value(employee.getId().toString()));
	}

	@Test
	void employeeCannotQueryAnotherUsersAttendance() throws Exception {
		User firstEmployee = persistUser("employee-4");
		User secondEmployee = persistUser("employee-5");
		Outlet outlet = persistOutlet("Outlet D", false);
		persistMembership(outlet, firstEmployee, OutletRole.EMPLOYEE, OutletMembershipStatus.ACCEPTED);
		persistMembership(outlet, secondEmployee, OutletRole.EMPLOYEE, OutletMembershipStatus.ACCEPTED);

		mockMvc.perform(get("/api/v1/outlets/{outletId}/attendance", outlet.getId())
				.header("Authorization", "Bearer " + jwtService.generateAccessToken(firstEmployee))
				.param("userId", secondEmployee.getId().toString()))
			.andExpect(status().isForbidden())
			.andExpect(jsonPath("$.code").value("FORBIDDEN"));
	}

	@Test
	void createOwnAttendanceRejectsInvalidLatitude() throws Exception {
		User employee = persistUser("employee-6");
		Outlet outlet = persistOutlet("Outlet E", false);
		persistMembership(outlet, employee, OutletRole.EMPLOYEE, OutletMembershipStatus.ACCEPTED);

		mockMvc.perform(post("/api/v1/outlets/{outletId}/attendance", outlet.getId())
				.header("Authorization", "Bearer " + jwtService.generateAccessToken(employee))
				.contentType(MediaType.APPLICATION_JSON)
				.content("""
					{
					  \"type\": \"CLOCK_IN\",
					  \"latitude\": 100.0000000,
					  \"longitude\": 77.5970000
					}
					"""))
			.andExpect(status().isBadRequest())
			.andExpect(jsonPath("$.code").value("VALIDATION_ERROR"));
	}

	private User persistUser(String authUidPrefix) {
		User user = new User();
		user.setAuthUid(authUidPrefix + "-uid");
		user.setName("Test User " + authUidPrefix);
		user.setEmail(authUidPrefix + "@example.com");
		user.setPhone("+911234567890");
		return userRepository.saveAndFlush(user);
	}

	private Outlet persistOutlet(String name, boolean geofenceEnabled) {
		Outlet outlet = new Outlet();
		outlet.setName(name);
		outlet.setLatitude(new BigDecimal("12.9715987"));
		outlet.setLongitude(new BigDecimal("77.5945627"));
		outlet.setRadiusMeters(150);
		outlet.setGeofenceEnabled(geofenceEnabled);
		return outletRepository.saveAndFlush(outlet);
	}

	private OutletMembership persistMembership(
		Outlet outlet,
		User user,
		OutletRole role,
		OutletMembershipStatus status
	) {
		OutletMembership membership = new OutletMembership();
		membership.setOutlet(outlet);
		membership.setUser(user);
		membership.setRole(role);
		membership.setStatus(status);
		return outletMembershipRepository.saveAndFlush(membership);
	}

	private AttendanceEntry persistAttendance(
		Outlet outlet,
		User user,
		AttendanceEntryType type,
		Instant entryTime
	) {
		AttendanceEntry entry = new AttendanceEntry();
		entry.setOutlet(outlet);
		entry.setUser(user);
		entry.setType(type);
		entry.setEntryTime(entryTime);
		entry.setLatitude(new BigDecimal("12.9715987"));
		entry.setLongitude(new BigDecimal("77.5945627"));
		return attendanceEntryRepository.saveAndFlush(entry);
	}
}
