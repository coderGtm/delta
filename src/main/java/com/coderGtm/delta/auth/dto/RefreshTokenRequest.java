package com.coderGtm.delta.auth.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

/**
 * Token refresh request containing the client's current refresh token.
 */
public record RefreshTokenRequest(
	@NotBlank
	@Size(max = 512)
	String refreshToken
) {
}
