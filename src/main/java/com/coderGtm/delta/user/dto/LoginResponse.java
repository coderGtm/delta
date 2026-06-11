package com.coderGtm.delta.user.dto;

import java.time.Instant;
import java.util.UUID;

public record LoginResponse(
	
	UUID id,

	String name,

	String email,

	String phone,

	String accessToken,

	String refreshToken,

	Instant createdAt,

	Instant updatedAt
	
) {}
