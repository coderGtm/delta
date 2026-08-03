package com.coderGtm.delta.common.web;

import jakarta.servlet.http.HttpServletRequest;

/**
 * Resolves the best-effort client IP address, honoring common proxy headers
 * before falling back to the servlet remote address.
 */
public final class ClientIpUtils {

	private ClientIpUtils() {
	}

	/**
	 * Resolves the originating client IP from forwarded headers when present.
	 */
	public static String resolve(HttpServletRequest request) {
		String forwardedFor = request.getHeader("X-Forwarded-For");
		if (forwardedFor != null && !forwardedFor.isBlank()) {
			return forwardedFor.split(",")[0].trim();
		}

		String realIp = request.getHeader("X-Real-IP");
		if (realIp != null && !realIp.isBlank()) {
			return realIp.trim();
		}

		return request.getRemoteAddr();
	}
}
