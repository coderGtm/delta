package com.coderGtm.delta.auth.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

public record LoginRequest(

	@NotBlank
	@Size(max = 255)
	String firebaseIdToken
	
) {}