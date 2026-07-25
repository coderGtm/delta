package com.coderGtm.delta.auth.dto;

/**
 * Response returned after a successful refresh-token rotation.
 */
public record RefreshTokenResponse(
	String accessToken,
	String refreshToken
) {
}
