package com.coderGtm.delta.auth.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import java.time.Instant;
import java.util.Optional;
import java.util.UUID;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import com.coderGtm.delta.auth.dto.FirebaseUserInfo;
import com.coderGtm.delta.auth.dto.LoginRequest;
import com.coderGtm.delta.auth.dto.LoginResponse;
import com.coderGtm.delta.auth.dto.LogoutRequest;
import com.coderGtm.delta.auth.dto.RefreshTokenRequest;
import com.coderGtm.delta.auth.dto.RefreshTokenResponse;
import com.coderGtm.delta.auth.mapper.AuthMapper;
import com.coderGtm.delta.common.exception.InvalidTokenException;
import com.coderGtm.delta.user.User;
import com.coderGtm.delta.user.UserRepository;
import com.coderGtm.delta.user.UserService;
import com.google.firebase.auth.FirebaseAuthException;

@ExtendWith(MockitoExtension.class)
class AuthServiceTest {

	@Mock
	private UserRepository userRepository;

	@Mock
	private FirebaseService firebaseService;

	@Mock
	private UserService userService;

	@Mock
	private JwtService jwtService;

	@Mock
	private RefreshTokenService refreshTokenService;

	@Mock
	private AuthMapper authMapper;

	private AuthService authService;

	@BeforeEach
	void setUp() {
		authService = new AuthService(
			userRepository,
			firebaseService,
			userService,
			jwtService,
			refreshTokenService,
			authMapper
		);
	}

	@Test
	void loginWithExistingUserReturnsMappedResponse() throws Exception {
		FirebaseUserInfo userInfo = new FirebaseUserInfo("auth-uid", "Gautam", "gautam@example.com", "+911234567890");
		User user = user("auth-uid", "Gautam", "gautam@example.com", "+911234567890");
		RefreshTokenService.IssuedRefreshToken issuedRefreshToken =
			new RefreshTokenService.IssuedRefreshToken(user, "refresh-token", Instant.now().plusSeconds(60));
		LoginResponse expected = new LoginResponse(
			user.getId(),
			user.getName(),
			user.getEmail(),
			user.getPhone(),
			"access-token",
			"refresh-token",
			user.getCreatedAt(),
			user.getUpdatedAt()
		);

		when(firebaseService.verifyIdToken("firebase-token")).thenReturn(userInfo);
		when(userRepository.findByAuthUid("auth-uid")).thenReturn(Optional.of(user));
		when(jwtService.generateAccessToken(user)).thenReturn("access-token");
		when(refreshTokenService.create(user)).thenReturn(issuedRefreshToken);
		when(authMapper.toResponse(user, "access-token", "refresh-token")).thenReturn(expected);

		LoginResponse response = authService.login(new LoginRequest("firebase-token"));

		assertThat(response).isEqualTo(expected);
		verify(userService, never()).createUser(anyString(), anyString(), anyString(), anyString());
		verify(authMapper).toResponse(user, "access-token", "refresh-token");
	}

	@Test
	void loginWithNewUserCreatesUserAndReturnsMappedResponse() throws Exception {
		FirebaseUserInfo userInfo = new FirebaseUserInfo("new-auth-uid", "New User", "new@example.com", "+911111111111");
		User user = user("new-auth-uid", "New User", "new@example.com", "+911111111111");
		RefreshTokenService.IssuedRefreshToken issuedRefreshToken =
			new RefreshTokenService.IssuedRefreshToken(user, "refresh-token", Instant.now().plusSeconds(60));
		LoginResponse expected = new LoginResponse(
			user.getId(),
			user.getName(),
			user.getEmail(),
			user.getPhone(),
			"access-token",
			"refresh-token",
			user.getCreatedAt(),
			user.getUpdatedAt()
		);

		when(firebaseService.verifyIdToken("firebase-token")).thenReturn(userInfo);
		when(userRepository.findByAuthUid("new-auth-uid")).thenReturn(Optional.empty());
		when(userService.createUser("new-auth-uid", "New User", "new@example.com", "+911111111111")).thenReturn(user);
		when(jwtService.generateAccessToken(user)).thenReturn("access-token");
		when(refreshTokenService.create(user)).thenReturn(issuedRefreshToken);
		when(authMapper.toResponse(user, "access-token", "refresh-token")).thenReturn(expected);

		LoginResponse response = authService.login(new LoginRequest("firebase-token"));

		assertThat(response).isEqualTo(expected);
		verify(userService).createUser("new-auth-uid", "New User", "new@example.com", "+911111111111");
	}

	@Test
	void loginWithInvalidFirebaseTokenThrowsInvalidTokenException() throws Exception {
		FirebaseAuthException firebaseAuthException = mock(FirebaseAuthException.class);
		when(firebaseService.verifyIdToken("firebase-token")).thenThrow(firebaseAuthException);

		assertThatThrownBy(() -> authService.login(new LoginRequest("firebase-token")))
			.isInstanceOf(InvalidTokenException.class)
			.hasMessage("Invalid Firebase ID Token")
			.hasCause(firebaseAuthException);

		verify(userRepository, never()).findByAuthUid(anyString());
		verify(jwtService, never()).generateAccessToken(org.mockito.ArgumentMatchers.any());
	}

	@Test
	void refreshRotatesTokenAndReturnsNewTokens() {
		User user = user("auth-uid", "Gautam", "gautam@example.com", "+911234567890");
		RefreshTokenService.IssuedRefreshToken rotated =
			new RefreshTokenService.IssuedRefreshToken(user, "new-refresh-token", Instant.now().plusSeconds(60));

		when(refreshTokenService.rotate("old-refresh-token")).thenReturn(rotated);
		when(jwtService.generateAccessToken(user)).thenReturn("new-access-token");

		RefreshTokenResponse response = authService.refresh(new RefreshTokenRequest("old-refresh-token"));

		assertThat(response).isEqualTo(new RefreshTokenResponse("new-access-token", "new-refresh-token"));
		verify(refreshTokenService).rotate("old-refresh-token");
	}

	@Test
	void logoutDelegatesToRefreshTokenService() {
		authService.logout(new LogoutRequest("refresh-token"));

		verify(refreshTokenService).revoke("refresh-token");
	}

	@Test
	void logoutAllDelegatesToRefreshTokenService() {
		User user = user("auth-uid", "Gautam", "gautam@example.com", "+911234567890");

		authService.logoutAll(user);

		verify(refreshTokenService).revokeAllForUser(user);
	}

	private User user(String authUid, String name, String email, String phone) {
		User user = new User();
		user.setId(UUID.randomUUID());
		user.setAuthUid(authUid);
		user.setName(name);
		user.setEmail(email);
		user.setPhone(phone);
		return user;
	}
}
