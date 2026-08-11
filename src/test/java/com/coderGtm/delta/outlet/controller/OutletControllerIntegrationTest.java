package com.coderGtm.delta.outlet.controller;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.put;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import java.math.BigDecimal;

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
class OutletControllerIntegrationTest {

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
	private JwtService jwtService;

	private MockMvc mockMvc;

	@BeforeEach
	void setUp() {
		mockMvc = MockMvcBuilders.webAppContextSetup(context)
			.addFilters(springSecurityFilterChain)
			.build();
	}

	@Test
	void updateOutletGeofencePersistsOwnerControlledSetting() throws Exception {
		User owner = persistUser("owner");
		Outlet outlet = persistOutlet("Outlet A", false);
		persistMembership(outlet, owner, OutletRole.OWNER, OutletMembershipStatus.ACCEPTED);

		mockMvc.perform(put("/api/v1/outlets/{outletId}/geofence", outlet.getId())
				.header("Authorization", "Bearer " + jwtService.generateAccessToken(owner))
				.contentType(MediaType.APPLICATION_JSON)
				.content("""
					{
					  \"geofenceEnabled\": true
					}
					"""))
			.andExpect(status().isOk())
			.andExpect(jsonPath("$.id").value(outlet.getId().toString()))
			.andExpect(jsonPath("$.geofenceEnabled").value(true));

		assertThat(outletRepository.findById(outlet.getId()).orElseThrow().isGeofenceEnabled()).isTrue();
	}

	@Test
	void updateMemberDisplayNamePersistsOwnerControlledNameForEmployee() throws Exception {
		User owner = persistUser("owner-display");
		User employee = persistUser("employee-display");
		Outlet outlet = persistOutlet("Outlet A", false);
		persistMembership(outlet, owner, OutletRole.OWNER, OutletMembershipStatus.ACCEPTED);
		OutletMembership employeeMembership = persistMembership(outlet, employee, OutletRole.EMPLOYEE, OutletMembershipStatus.ACCEPTED);

		mockMvc.perform(put("/api/v1/outlets/{outletId}/memberships/{membershipId}/display-name", outlet.getId(), employeeMembership.getId())
				.header("Authorization", "Bearer " + jwtService.generateAccessToken(owner))
				.contentType(MediaType.APPLICATION_JSON)
				.content("""
					{
					  \"displayName\": \"Nickname\"
					}
					"""))
			.andExpect(status().isOk())
			.andExpect(jsonPath("$.membershipId").value(employeeMembership.getId().toString()))
			.andExpect(jsonPath("$.userId").value(employee.getId().toString()))
			.andExpect(jsonPath("$.displayName").value("Nickname"));

		assertThat(outletMembershipRepository.findDetailedByIdAndRemovedAtIsNull(employeeMembership.getId())
			.orElseThrow()
			.getDisplayName()).isEqualTo("Nickname");
	}

	@Test
	void getMyOutletsReturnsPaginatedResponse() throws Exception {
		User user = persistUser("employee");
		Outlet firstOutlet = persistOutlet("Outlet A", false);
		Outlet secondOutlet = persistOutlet("Outlet B", true);
		persistMembership(firstOutlet, user, OutletRole.EMPLOYEE, OutletMembershipStatus.ACCEPTED);
		persistMembership(secondOutlet, user, OutletRole.EMPLOYEE, OutletMembershipStatus.ACCEPTED);

		mockMvc.perform(get("/api/v1/outlets/mine")
				.header("Authorization", "Bearer " + jwtService.generateAccessToken(user))
				.param("page", "0")
				.param("size", "1"))
			.andExpect(status().isOk())
			.andExpect(jsonPath("$.content.length()").value(1))
			.andExpect(jsonPath("$.page").value(0))
			.andExpect(jsonPath("$.size").value(1))
			.andExpect(jsonPath("$.totalElements").value(2))
			.andExpect(jsonPath("$.totalPages").value(2));
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
		membership.setDisplayName(user.getName());
		membership.setRole(role);
		membership.setStatus(status);
		return outletMembershipRepository.saveAndFlush(membership);
	}
}
