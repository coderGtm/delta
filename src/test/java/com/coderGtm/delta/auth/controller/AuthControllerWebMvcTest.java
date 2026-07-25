package com.coderGtm.delta.auth.controller;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import java.time.Instant;
import java.util.UUID;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;
import org.springframework.validation.beanvalidation.LocalValidatorFactoryBean;

import com.coderGtm.delta.auth.dto.LoginRequest;
import com.coderGtm.delta.auth.dto.LoginResponse;
import com.coderGtm.delta.auth.dto.RefreshTokenRequest;
import com.coderGtm.delta.auth.dto.RefreshTokenResponse;
import com.coderGtm.delta.auth.service.AuthService;
import com.coderGtm.delta.common.exception.GlobalExceptionHandler;
import com.coderGtm.delta.common.exception.InvalidTokenException;
import com.coderGtm.delta.user.User;

@ExtendWith(MockitoExtension.class)
class AuthControllerWebMvcTest {

	@Mock
	private AuthService authService;

	private MockMvc mockMvc;
	private AuthController authController;

	@BeforeEach
	void setUp() {
		authController = new AuthController(authService);

		LocalValidatorFactoryBean validator = new LocalValidatorFactoryBean();
		validator.afterPropertiesSet();

		mockMvc = MockMvcBuilders.standaloneSetup(authController)
			.setControllerAdvice(new GlobalExceptionHandler())
			.setValidator(validator)
			.build();
	}

	@Test
	void loginReturnsResponseBody() throws Exception {
		UUID userId = UUID.randomUUID();
		when(authService.login(any(LoginRequest.class))).thenReturn(new LoginResponse(
			userId,
			"Gautam",
			"gautam@example.com",
			"+911234567890",
			"access-token",
			"refresh-token",
			Instant.parse("2024-01-01T00:00:00Z"),
			Instant.parse("2024-01-02T00:00:00Z")
		));

		mockMvc.perform(post("/api/v1/auth/login")
				.contentType(MediaType.APPLICATION_JSON)
				.content("""
					{
					  \"firebaseIdToken\": \"firebase-token\"
					}
					"""))
			.andExpect(status().isOk())
			.andExpect(jsonPath("$.id").value(userId.toString()))
			.andExpect(jsonPath("$.accessToken").value("access-token"))
			.andExpect(jsonPath("$.refreshToken").value("refresh-token"));
	}

	@Test
	void refreshReturnsResponseBody() throws Exception {
		when(authService.refresh(any(RefreshTokenRequest.class)))
			.thenReturn(new RefreshTokenResponse("new-access-token", "new-refresh-token"));

		mockMvc.perform(post("/api/v1/auth/refresh")
				.contentType(MediaType.APPLICATION_JSON)
				.content("""
					{
					  \"refreshToken\": \"refresh-token\"
					}
					"""))
			.andExpect(status().isOk())
			.andExpect(jsonPath("$.accessToken").value("new-access-token"))
			.andExpect(jsonPath("$.refreshToken").value("new-refresh-token"));
	}

	@Test
	void logoutReturnsNoContent() throws Exception {
		mockMvc.perform(post("/api/v1/auth/logout")
				.contentType(MediaType.APPLICATION_JSON)
				.content("""
					{
					  \"refreshToken\": \"refresh-token\"
					}
					"""))
			.andExpect(status().isNoContent());

		verify(authService).logout(any());
	}

	@Test
	void logoutAllDelegatesToService() {
		User user = new User();
		user.setId(UUID.randomUUID());
		user.setName("Gautam");

		ResponseEntity<Void> response = authController.logoutAll(user);

		verify(authService).logoutAll(user);
		org.assertj.core.api.Assertions.assertThat(response.getStatusCode().value()).isEqualTo(204);
	}

	@Test
	void loginRejectsBlankFirebaseToken() throws Exception {
		mockMvc.perform(post("/api/v1/auth/login")
				.contentType(MediaType.APPLICATION_JSON)
				.content("""
					{
					  \"firebaseIdToken\": \"\"
					}
					"""))
			.andExpect(status().isBadRequest());

		verify(authService, never()).login(any());
	}

	@Test
	void refreshRejectsBlankRefreshToken() throws Exception {
		mockMvc.perform(post("/api/v1/auth/refresh")
				.contentType(MediaType.APPLICATION_JSON)
				.content("""
					{
					  \"refreshToken\": \"\"
					}
					"""))
			.andExpect(status().isBadRequest());

		verify(authService, never()).refresh(any());
	}

	@Test
	void logoutRejectsBlankRefreshToken() throws Exception {
		mockMvc.perform(post("/api/v1/auth/logout")
				.contentType(MediaType.APPLICATION_JSON)
				.content("""
					{
					  \"refreshToken\": \"\"
					}
					"""))
			.andExpect(status().isBadRequest());

		verify(authService, never()).logout(any());
	}

	@Test
	void invalidTokenExceptionReturnsStructuredUnauthorizedResponse() throws Exception {
		when(authService.refresh(any(RefreshTokenRequest.class)))
			.thenThrow(new InvalidTokenException("Invalid refresh token"));

		mockMvc.perform(post("/api/v1/auth/refresh")
				.contentType(MediaType.APPLICATION_JSON)
				.content("""
					{
					  \"refreshToken\": \"refresh-token\"
					}
					"""))
			.andExpect(status().isUnauthorized())
			.andExpect(jsonPath("$.code").value("INVALID_TOKEN"))
			.andExpect(jsonPath("$.message").value("Invalid refresh token"))
			.andExpect(jsonPath("$.timestamp").exists());
	}
}
