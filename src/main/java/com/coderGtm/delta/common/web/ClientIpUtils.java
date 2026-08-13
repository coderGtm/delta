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
	 * Resolves the originating client IP, honoring proxy headers by default.
	 */
	public static String resolve(HttpServletRequest request) {
		return resolve(request, true);
	}

	/**
	 * Resolves the originating client IP from forwarded headers only when
	 * {@code trustProxyHeaders} is true, otherwise uses the socket remote
	 * address to prevent header spoofing.
	 */
	public static String resolve(HttpServletRequest request, boolean trustProxyHeaders) {
		if (trustProxyHeaders) {
			String forwardedFor = request.getHeader("X-Forwarded-For");
			if (forwardedFor != null && !forwardedFor.isBlank()) {
				return forwardedFor.split(",")[0].trim();
			}

			String realIp = request.getHeader("X-Real-IP");
			if (realIp != null && !realIp.isBlank()) {
				return realIp.trim();
			}
		}

		return request.getRemoteAddr();
	}
}
