package com.coderGtm.delta.auth.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

/**
 * Login request carrying the Firebase-issued ID token from the client.
 */
public record LoginRequest(
	@NotBlank
	@Size(max = 255)
	String firebaseIdToken
) {
}
