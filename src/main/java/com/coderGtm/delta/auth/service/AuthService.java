package com.coderGtm.delta.auth.service;

import java.util.Map;

import org.springframework.stereotype.Service;

import com.coderGtm.delta.auth.dto.FirebaseUserInfo;
import com.coderGtm.delta.auth.dto.LoginRequest;
import com.coderGtm.delta.auth.dto.LoginResponse;
import com.coderGtm.delta.auth.dto.LogoutRequest;
import com.coderGtm.delta.auth.dto.RefreshTokenRequest;
import com.coderGtm.delta.auth.dto.RefreshTokenResponse;
import com.coderGtm.delta.auth.mapper.AuthMapper;
import com.coderGtm.delta.common.audit.service.AuditService;
import com.coderGtm.delta.common.exception.InvalidTokenException;
import com.coderGtm.delta.common.metrics.ApplicationMetrics;
import com.coderGtm.delta.user.User;
import com.coderGtm.delta.user.UserRepository;
import com.coderGtm.delta.user.UserService;
import com.google.firebase.auth.FirebaseAuthException;

import lombok.RequiredArgsConstructor;

/**
 * Coordinates the application's login, refresh, and logout flows.
 */
@Service
@RequiredArgsConstructor
public class AuthService {

	private final UserRepository userRepository;
	private final FirebaseService firebaseService;
	private final UserService userService;
	private final JwtService jwtService;
	private final RefreshTokenService refreshTokenService;
	private final AuthMapper authMapper;
	private final AuditService auditService;
	private final ApplicationMetrics applicationMetrics;

	/**
	 * Verifies a Firebase ID token, creates the local user when needed, and
	 * issues the application's token pair.
	 */
	public LoginResponse login(LoginRequest request) {
		FirebaseUserInfo userInfo;

		try {
			userInfo = firebaseService.verifyIdToken(request.firebaseIdToken());
		} catch (FirebaseAuthException e) {
			throw new InvalidTokenException("Invalid Firebase ID Token", e);
		}

		User user = userRepository
			.findByAuthUid(userInfo.uid())
			.orElseGet(() -> userService.createUser(
				userInfo.uid(),
				userInfo.name(),
				userInfo.email(),
				userInfo.phoneNumber()
			));

		String accessToken = jwtService.generateAccessToken(user);
		RefreshTokenService.IssuedRefreshToken issuedRefreshToken = refreshTokenService.create(user);

		applicationMetrics.increment("auth.login.success");
		auditService.record(user.getId(), "AUTH_LOGIN", "USER", user.getId(), Map.of("email", user.getEmail()));

		return authMapper.toResponse(user, accessToken, issuedRefreshToken.refreshToken());
	}

	/**
	 * Rotates a refresh token and returns a fresh access token plus refresh token.
	 */
	public RefreshTokenResponse refresh(RefreshTokenRequest request) {
		RefreshTokenService.IssuedRefreshToken rotatedToken = refreshTokenService.rotate(request.refreshToken());
		String accessToken = jwtService.generateAccessToken(rotatedToken.user());

		applicationMetrics.increment("auth.refresh.success");
		auditService.record(rotatedToken.user().getId(), "AUTH_REFRESH", "USER", rotatedToken.user().getId(), Map.of());

		return new RefreshTokenResponse(accessToken, rotatedToken.refreshToken());
	}

	/**
	 * Revokes a single refresh token.
	 */
	public void logout(LogoutRequest request) {
		refreshTokenService.revoke(request.refreshToken());
		applicationMetrics.increment("auth.logout.success");
	}

	/**
	 * Revokes every active refresh token owned by the authenticated user.
	 */
	public void logoutAll(User user) {
		refreshTokenService.revokeAllForUser(user);
		applicationMetrics.increment("auth.logout_all.success");
		auditService.record(user.getId(), "AUTH_LOGOUT_ALL", "USER", user.getId(), Map.of());
	}
}
