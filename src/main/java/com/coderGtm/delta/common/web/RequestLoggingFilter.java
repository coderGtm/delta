package com.coderGtm.delta.common.web;

import java.io.IOException;
import java.util.UUID;

import org.slf4j.MDC;
import org.springframework.security.core.Authentication;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.stereotype.Component;
import org.springframework.web.filter.OncePerRequestFilter;

import com.coderGtm.delta.user.User;

import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;

/**
 * Adds request correlation metadata and structured request completion logs for
 * every HTTP request.
 */
@Component
@Slf4j
@RequiredArgsConstructor
public class RequestLoggingFilter extends OncePerRequestFilter {

	private static final String REQUEST_ID_HEADER = "X-Request-Id";

	private final WebSecurityProperties webSecurityProperties;

	@Override
	protected void doFilterInternal(
		HttpServletRequest request,
		HttpServletResponse response,
		FilterChain filterChain
	) throws ServletException, IOException {
		String requestId = resolveRequestId(request);
		long startedAt = System.nanoTime();

		MDC.put("requestId", requestId);
		response.setHeader(REQUEST_ID_HEADER, requestId);

		try {
			filterChain.doFilter(request, response);
		} finally {
			String userId = resolveAuthenticatedUserId();
			if (userId != null) {
				MDC.put("userId", userId);
			}

			long durationMs = (System.nanoTime() - startedAt) / 1_000_000;
			log.info(
				"HTTP {} {} completed with status={} durationMs={} clientIp={} requestId={} userId={}",
				request.getMethod(),
				request.getRequestURI(),
				response.getStatus(),
				durationMs,
				ClientIpUtils.resolve(request, webSecurityProperties.trustProxyHeaders()),
				requestId,
				userId
			);

			MDC.remove("requestId");
			MDC.remove("userId");
		}
	}

	private String resolveRequestId(HttpServletRequest request) {
		String requestId = request.getHeader(REQUEST_ID_HEADER);
		return requestId == null || requestId.isBlank() ? UUID.randomUUID().toString() : requestId.trim();
	}

	private String resolveAuthenticatedUserId() {
		Authentication authentication = SecurityContextHolder.getContext().getAuthentication();
		if (authentication == null || !(authentication.getPrincipal() instanceof User user)) {
			return null;
		}

		return user.getId() != null ? user.getId().toString() : null;
	}
}
