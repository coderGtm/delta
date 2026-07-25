package com.coderGtm.delta.auth.controller;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import java.util.List;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.MvcResult;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.web.context.WebApplicationContext;

import jakarta.servlet.Filter;

import com.coderGtm.delta.TestApplication;
import com.coderGtm.delta.TestFirebaseConfiguration;
import com.coderGtm.delta.auth.repository.RefreshTokenRepository;
import com.coderGtm.delta.auth.service.JwtService;
import com.coderGtm.delta.auth.service.RefreshTokenService;
import com.coderGtm.delta.common.exception.InvalidTokenException;
import com.coderGtm.delta.user.User;
import com.coderGtm.delta.user.UserRepository;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

@SpringBootTest(classes = { TestApplication.class, TestFirebaseConfiguration.class })
@Transactional
class AuthTokenFlowIntegrationTest {

	@Autowired
	private WebApplicationContext context;

	@Autowired
	private UserRepository userRepository;

	@Autowired
	private RefreshTokenRepository refreshTokenRepository;

	@Autowired
	private RefreshTokenService refreshTokenService;

	@Autowired
	private JwtService jwtService;

	@Autowired
	private Filter springSecurityFilterChain;

	private final ObjectMapper objectMapper = new ObjectMapper();

	private MockMvc mockMvc;

	@BeforeEach
	void setUp() {
		mockMvc = MockMvcBuilders.webAppContextSetup(context)
			.addFilters(springSecurityFilterChain)
			.build();
	}

	@Test
	void refreshEndpointRotatesRefreshTokenAndReturnsNewTokens() throws Exception {
		User user = persistUser("refresh-user");
		RefreshTokenService.IssuedRefreshToken issued = refreshTokenService.create(user);

		MvcResult result = mockMvc.perform(post("/api/v1/auth/refresh")
				.contentType(MediaType.APPLICATION_JSON)
				.content("""
					{
					  \"refreshToken\": \"%s\"
					}
					""".formatted(issued.refreshToken())))
			.andExpect(status().isOk())
			.andExpect(jsonPath("$.accessToken").isNotEmpty())
			.andExpect(jsonPath("$.refreshToken").isNotEmpty())
			.andReturn();

		JsonNode json = objectMapper.readTree(result.getResponse().getContentAsString());
		String rotatedRefreshToken = json.get("refreshToken").asText();

		assertThat(rotatedRefreshToken).isNotEqualTo(issued.refreshToken());
		assertThat(refreshTokenRepository.findAll()).hasSize(2);
		assertThatThrownBy(() -> refreshTokenService.validate(issued.refreshToken()))
			.isInstanceOf(InvalidTokenException.class);
		assertThat(refreshTokenService.validate(rotatedRefreshToken).getUser().getId()).isEqualTo(user.getId());
	}

	@Test
	void refreshEndpointReturnsUnauthorizedForInvalidToken() throws Exception {
		mockMvc.perform(post("/api/v1/auth/refresh")
				.contentType(MediaType.APPLICATION_JSON)
				.content("""
					{
					  \"refreshToken\": \"does-not-exist\"
					}
					"""))
			.andExpect(status().isUnauthorized())
			.andExpect(jsonPath("$.code").value("INVALID_TOKEN"))
			.andExpect(jsonPath("$.message").value("Invalid refresh token"));
	}

	@Test
	void logoutEndpointRevokesSubmittedRefreshToken() throws Exception {
		User user = persistUser("logout-user");
		RefreshTokenService.IssuedRefreshToken issued = refreshTokenService.create(user);

		mockMvc.perform(post("/api/v1/auth/logout")
				.contentType(MediaType.APPLICATION_JSON)
				.content("""
					{
					  \"refreshToken\": \"%s\"
					}
					""".formatted(issued.refreshToken())))
			.andExpect(status().isNoContent());

		assertThatThrownBy(() -> refreshTokenService.validate(issued.refreshToken()))
			.isInstanceOf(InvalidTokenException.class)
			.hasMessage("Refresh token has been revoked");
	}

	@Test
	void logoutAllRejectsUnauthenticatedRequests() throws Exception {
		mockMvc.perform(post("/api/v1/auth/logout-all"))
			.andExpect(status().isForbidden());
	}

	@Test
	void logoutAllRevokesOnlyAuthenticatedUsersRefreshTokens() throws Exception {
		User currentUser = persistUser("current-user");
		User otherUser = persistUser("other-user");

		refreshTokenService.create(currentUser);
		refreshTokenService.create(currentUser);
		refreshTokenService.create(otherUser);

		String accessToken = jwtService.generateAccessToken(currentUser);

		mockMvc.perform(post("/api/v1/auth/logout-all")
				.header("Authorization", "Bearer " + accessToken))
			.andExpect(status().isNoContent());

		List<com.coderGtm.delta.auth.entity.RefreshToken> currentUsersTokens = refreshTokenRepository.findAll().stream()
			.filter(token -> token.getUser().getId().equals(currentUser.getId()))
			.toList();
		List<com.coderGtm.delta.auth.entity.RefreshToken> otherUsersTokens = refreshTokenRepository.findAll().stream()
			.filter(token -> token.getUser().getId().equals(otherUser.getId()))
			.toList();

		assertThat(currentUsersTokens).allMatch(com.coderGtm.delta.auth.entity.RefreshToken::isRevoked);
		assertThat(otherUsersTokens).allMatch(token -> !token.isRevoked());
	}

	private User persistUser(String authUidPrefix) {
		User user = new User();
		user.setAuthUid(authUidPrefix + "-uid");
		user.setName("Test User");
		user.setEmail(authUidPrefix + "@example.com");
		user.setPhone("+911234567890");
		return userRepository.saveAndFlush(user);
	}
}
