package com.coderGtm.delta.common.web;

import java.io.IOException;
import java.time.Duration;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import java.util.Objects;

import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.AtomicInteger;

import org.springframework.http.MediaType;
import org.springframework.security.authentication.AnonymousAuthenticationToken;
import org.springframework.security.core.Authentication;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.stereotype.Component;
import org.springframework.web.filter.OncePerRequestFilter;
import org.springframework.util.AntPathMatcher;

import com.coderGtm.delta.common.dto.ErrorResponse;
import com.coderGtm.delta.user.User;
import com.fasterxml.jackson.databind.ObjectMapper;

import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import lombok.RequiredArgsConstructor;

/**
 * Provides lightweight in-memory rate limiting for high-risk authentication and
 * write-heavy business endpoints.
 *
 * <p>This implementation is suitable for a single application instance. If the
 * service is later scaled horizontally, move these limits to Redis or an API
 * gateway.</p>
 */
@Component
@RequiredArgsConstructor
public class RateLimitingFilter extends OncePerRequestFilter {

	private final ObjectMapper objectMapper = new ObjectMapper();
	private final AntPathMatcher pathMatcher = new AntPathMatcher();
	private final Map<String, WindowCounter> counters = new ConcurrentHashMap<>();

	private final List<RateLimitPolicy> policies = List.of(
		new RateLimitPolicy("POST", ApiPaths.AUTH + "/login", 10, Duration.ofMinutes(1), KeyStrategy.IP),
		new RateLimitPolicy("POST", ApiPaths.AUTH + "/refresh", 30, Duration.ofMinutes(1), KeyStrategy.IP),
		new RateLimitPolicy("POST", ApiPaths.AUTH + "/logout", 30, Duration.ofMinutes(1), KeyStrategy.IP),
		new RateLimitPolicy("POST", ApiPaths.AUTH + "/logout-all", 30, Duration.ofMinutes(1), KeyStrategy.USER_OR_IP),
		new RateLimitPolicy("POST", ApiPaths.OUTLETS + "/*/memberships/invite", 20, Duration.ofMinutes(1), KeyStrategy.USER_OR_IP),
		new RateLimitPolicy("POST", ApiPaths.OUTLETS + "/*/attendance", 20, Duration.ofMinutes(1), KeyStrategy.USER_OR_IP),
		new RateLimitPolicy("POST", ApiPaths.OUTLETS + "/*/attendance/manage", 60, Duration.ofMinutes(1), KeyStrategy.USER_OR_IP),
		new RateLimitPolicy("PUT", ApiPaths.OUTLETS + "/*/attendance/*", 60, Duration.ofMinutes(1), KeyStrategy.USER_OR_IP),
		new RateLimitPolicy("PUT", ApiPaths.OUTLETS + "/*/geofence", 20, Duration.ofMinutes(1), KeyStrategy.USER_OR_IP),
		new RateLimitPolicy("GET", ApiPaths.OUTLETS + "/*/reports/salary", 30, Duration.ofMinutes(1), KeyStrategy.USER_OR_IP),
		new RateLimitPolicy("GET", ApiPaths.OUTLETS + "/*/reports/salary.xlsx", 10, Duration.ofMinutes(1), KeyStrategy.USER_OR_IP)
	);

	@Override
	protected void doFilterInternal(
		HttpServletRequest request,
		HttpServletResponse response,
		FilterChain filterChain
	) throws ServletException, IOException {
		RateLimitPolicy policy = findMatchingPolicy(request);
		if (policy == null) {
			filterChain.doFilter(request, response);
			return;
		}

		String key = policy.keyStrategy().resolve(request);
		String counterKey = policy.httpMethod() + ':' + policy.pathPattern() + ':' + key;
		WindowCounter counter = counters.compute(counterKey, (ignored, existing) -> nextCounter(existing, policy));

		if (counter.currentCount().get() > policy.limit()) {
			writeTooManyRequests(response, counter.retryAfterSeconds());
			return;
		}

		filterChain.doFilter(request, response);
	}

	private RateLimitPolicy findMatchingPolicy(HttpServletRequest request) {
		String requestUri = request.getRequestURI();
		String method = request.getMethod();

		for (RateLimitPolicy policy : policies) {
			if (policy.httpMethod().equalsIgnoreCase(method) && pathMatcher.match(policy.pathPattern(), requestUri)) {
				return policy;
			}
		}

		return null;
	}

	private WindowCounter nextCounter(WindowCounter existing, RateLimitPolicy policy) {
		Instant now = Instant.now();
		if (existing == null || now.isAfter(existing.windowEndsAt())) {
			return new WindowCounter(now.plus(policy.window()), new AtomicInteger(1));
		}

		existing.currentCount().incrementAndGet();
		return existing;
	}

	private void writeTooManyRequests(HttpServletResponse response, long retryAfterSeconds) throws IOException {
		response.setStatus(429);
		response.setContentType(MediaType.APPLICATION_JSON_VALUE);
		response.setHeader("Retry-After", String.valueOf(retryAfterSeconds));
		objectMapper.writeValue(
			response.getWriter(),
			new ErrorResponse("RATE_LIMIT_EXCEEDED", "Too many requests. Please retry later.", Instant.now())
		);
	}

	private enum KeyStrategy {
		IP {
			@Override
			String resolve(HttpServletRequest request) {
				return ClientIpUtils.resolve(request);
			}
		},
		USER_OR_IP {
			@Override
			String resolve(HttpServletRequest request) {
				Authentication authentication = SecurityContextHolder.getContext().getAuthentication();
				if (authentication != null
					&& authentication.isAuthenticated()
					&& !(authentication instanceof AnonymousAuthenticationToken)
					&& authentication.getPrincipal() instanceof User user
					&& user.getId() != null) {
					return user.getId().toString();
				}
				return ClientIpUtils.resolve(request);
			}
		};

		abstract String resolve(HttpServletRequest request);
	}

	private record RateLimitPolicy(
		String httpMethod,
		String pathPattern,
		int limit,
		Duration window,
		KeyStrategy keyStrategy
	) {
		RateLimitPolicy {
			Objects.requireNonNull(httpMethod);
			Objects.requireNonNull(pathPattern);
			Objects.requireNonNull(window);
			Objects.requireNonNull(keyStrategy);
		}
	}

	private record WindowCounter(Instant windowEndsAt, AtomicInteger currentCount) {
		long retryAfterSeconds() {
			return Math.max(1, Duration.between(Instant.now(), windowEndsAt).getSeconds());
		}
	}
}
