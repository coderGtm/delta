package com.coderGtm.delta.auth.mapper;

import org.springframework.stereotype.Component;

import com.coderGtm.delta.auth.dto.LoginResponse;
import com.coderGtm.delta.user.User;

/**
 * Maps authentication domain objects into outward-facing API responses.
 */
@Component
public class AuthMapper {

	/**
	 * Builds the login response payload returned after a successful login.
	 */
	public LoginResponse toResponse(User user, String accessToken, String refreshToken) {
		return new LoginResponse(
			user.getId(),
			user.getName(),
			user.getEmail(),
			user.getPhone(),
			accessToken,
			refreshToken,
			user.getCreatedAt(),
			user.getUpdatedAt()
		);
	}
}
