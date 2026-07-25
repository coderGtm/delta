package com.coderGtm.delta.common.web;

/**
 * Centralizes versioned API base paths used by controllers and security
 * configuration.
 */
public final class ApiPaths {

	public static final String API_V1 = "/api/v1";
	public static final String AUTH = API_V1 + "/auth";
	public static final String OUTLETS = API_V1 + "/outlets";

	private ApiPaths() {
	}
}
