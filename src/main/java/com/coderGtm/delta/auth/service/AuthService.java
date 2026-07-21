package com.coderGtm.delta.auth.service;

import org.springframework.stereotype.Service;

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

import lombok.RequiredArgsConstructor;

@Service
@RequiredArgsConstructor
public class AuthService {

	private final UserRepository userRepository;
	private final FirebaseService firebaseService;
	private final UserService userService;
	private final JwtService jwtService;
	private final RefreshTokenService refreshTokenService;
	private final AuthMapper authMapper;

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

		return authMapper.toResponse(user, accessToken, issuedRefreshToken.refreshToken());
	}

	public RefreshTokenResponse refresh(RefreshTokenRequest request) {
		RefreshTokenService.IssuedRefreshToken rotatedToken = refreshTokenService.rotate(request.refreshToken());
		String accessToken = jwtService.generateAccessToken(rotatedToken.user());

		return new RefreshTokenResponse(accessToken, rotatedToken.refreshToken());
	}

	public void logout(LogoutRequest request) {
		refreshTokenService.revoke(request.refreshToken());
	}

	public void logoutAll(User user) {
		refreshTokenService.revokeAllForUser(user);
	}
}
