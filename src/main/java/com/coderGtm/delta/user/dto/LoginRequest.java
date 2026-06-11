package com.coderGtm.delta.user.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

public record LoginRequest(

	@NotBlank
	@Size(max = 255)
	String firebaseIdToken
	
) {}