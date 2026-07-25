package com.coderGtm.delta.auth.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.util.UUID;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.test.util.ReflectionTestUtils;

import com.coderGtm.delta.user.User;

class JwtServiceTest {

	private static final String SECRET = "1234567890123456789012345678901234567890123456789012345678901234";

	private JwtService jwtService;

	@BeforeEach
	void setUp() {
		jwtService = new JwtService();
		ReflectionTestUtils.setField(jwtService, "secret", SECRET);
		ReflectionTestUtils.setField(jwtService, "accessTokenExpiration", 60_000L);
	}

	@Test
	void generateAccessTokenRoundTripsUserId() {
		User user = user();

		String token = jwtService.generateAccessToken(user);

		assertThat(jwtService.extractUserId(token)).isEqualTo(user.getId());
		assertThat(jwtService.isTokenValid(token)).isTrue();
	}

	@Test
	void isTokenValidReturnsFalseForTamperedToken() {
		User user = user();
		String token = jwtService.generateAccessToken(user);

		assertThat(jwtService.isTokenValid(token + "tampered")).isFalse();
	}

	@Test
	void isTokenValidReturnsFalseForMalformedToken() {
		assertThat(jwtService.isTokenValid("not-a-jwt")).isFalse();
	}

	@Test
	void isTokenValidReturnsFalseForExpiredToken() {
		ReflectionTestUtils.setField(jwtService, "accessTokenExpiration", -1_000L);
		String expiredToken = jwtService.generateAccessToken(user());

		assertThat(jwtService.isTokenValid(expiredToken)).isFalse();
	}

	@Test
	void extractUserIdThrowsForMalformedToken() {
		assertThatThrownBy(() -> jwtService.extractUserId("not-a-jwt"))
			.isInstanceOf(RuntimeException.class);
	}

	private User user() {
		User user = new User();
		user.setId(UUID.randomUUID());
		user.setName("Gautam");
		user.setEmail("gautam@example.com");
		user.setPhone("+911234567890");
		return user;
	}
}
