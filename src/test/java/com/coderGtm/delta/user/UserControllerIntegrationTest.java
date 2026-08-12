package com.coderGtm.delta.user;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.delete;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.bean.override.mockito.MockitoSpyBean;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.web.context.WebApplicationContext;

import com.coderGtm.delta.TestApplication;
import com.coderGtm.delta.TestFirebaseConfiguration;
import com.coderGtm.delta.auth.service.FirebaseService;
import com.coderGtm.delta.auth.service.JwtService;
import com.google.firebase.auth.FirebaseAuth;
import com.google.firebase.auth.FirebaseAuthException;

import jakarta.servlet.Filter;

@SpringBootTest(classes = { TestApplication.class, TestFirebaseConfiguration.class })
@Transactional
class UserControllerIntegrationTest {

	@Autowired
	private WebApplicationContext context;

	@Autowired
	private Filter springSecurityFilterChain;

	@Autowired
	private UserRepository userRepository;

	@Autowired
	private JwtService jwtService;

	@MockitoSpyBean
	private FirebaseService firebaseService;

	@Autowired
	private FirebaseAuth firebaseAuth;

	private MockMvc mockMvc;

	@BeforeEach
	void setUp() {
		mockMvc = MockMvcBuilders.webAppContextSetup(context)
			.addFilters(springSecurityFilterChain)
			.build();
	}

	@Test
	void deleteAccountSoftDeletesUserAndClearsEmail() throws Exception {
		User user = persistUser("delete-me");
		user.setEmail("delete-me@example.com");
		userRepository.saveAndFlush(user);

		mockMvc.perform(delete("/api/v1/users/me")
				.header("Authorization", "Bearer " + jwtService.generateAccessToken(user)))
			.andExpect(status().isNoContent());

		User deleted = userRepository.findById(user.getId()).orElseThrow();
		assertThat(deleted.getDeletedAt()).isNotNull();
		assertThat(deleted.getEmail()).isNull();
		assertThat(deleted.getHistoricalEmail()).isEqualTo("delete-me@example.com");
		verify(firebaseService).deleteUser("delete-me-uid");
	}

	@Test
	void deleteAccountPropagatesFirebaseFailureAsConflict() throws Exception {
		User user = persistUser("delete-fail");
		user.setEmail("delete-fail@example.com");
		userRepository.saveAndFlush(user);

		org.mockito.Mockito.doThrow(mock(FirebaseAuthException.class))
			.when(firebaseService)
			.deleteUser("delete-fail-uid");

		mockMvc.perform(delete("/api/v1/users/me")
				.header("Authorization", "Bearer " + jwtService.generateAccessToken(user)))
			.andExpect(status().isConflict());

		User notDeleted = userRepository.findById(user.getId()).orElseThrow();
		assertThat(notDeleted.getDeletedAt()).isNull();
		assertThat(notDeleted.getEmail()).isEqualTo("delete-fail@example.com");
		assertThat(notDeleted.getHistoricalEmail()).isNull();
	}

	@Test
	void deleteAccountRejectsUnauthenticatedRequests() throws Exception {
		mockMvc.perform(delete("/api/v1/users/me"))
			.andExpect(status().isForbidden());
	}

	private User persistUser(String authUidPrefix) {
		User user = new User();
		user.setAuthUid(authUidPrefix + "-uid");
		user.setName("Test User " + authUidPrefix);
		user.setEmail(authUidPrefix + "@example.com");
		user.setPhone("+911234567890");
		return userRepository.saveAndFlush(user);
	}
}
