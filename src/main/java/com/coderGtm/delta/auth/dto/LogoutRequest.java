package com.coderGtm.delta.auth.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

/**
 * Logout request identifying the refresh token to revoke.
 */
public record LogoutRequest(
	@NotBlank
	@Size(max = 512)
	String refreshToken
) {
}
