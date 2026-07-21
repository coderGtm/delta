package com.coderGtm.delta.auth.dto;

public record RefreshTokenResponse(

	String accessToken,

	String refreshToken
	
) {}
