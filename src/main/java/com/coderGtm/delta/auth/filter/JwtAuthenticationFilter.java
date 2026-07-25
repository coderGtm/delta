package com.coderGtm.delta.auth.filter;

import java.io.IOException;
import java.util.List;
import java.util.UUID;

import org.springframework.security.authentication.UsernamePasswordAuthenticationToken;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.stereotype.Component;
import org.springframework.web.filter.OncePerRequestFilter;

import com.coderGtm.delta.auth.service.JwtService;
import com.coderGtm.delta.user.User;
import com.coderGtm.delta.user.UserRepository;

import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import lombok.RequiredArgsConstructor;

/**
 * Resolves the authenticated user from a bearer JWT and places that user in the
 * Spring Security context for the duration of the request.
 */
@Component
@RequiredArgsConstructor
public class JwtAuthenticationFilter extends OncePerRequestFilter {

	private final JwtService jwtService;
	private final UserRepository userRepository;

	/**
	 * Attempts bearer-token authentication and always continues the filter chain.
	 *
	 * <p>Authentication failures are intentionally treated as anonymous requests so
	 * that Spring Security can reject protected endpoints in a standard way.</p>
	 */
	@Override
	protected void doFilterInternal(
		HttpServletRequest request,
		HttpServletResponse response,
		FilterChain filterChain
	) throws ServletException, IOException {
		String authHeader = request.getHeader("Authorization");

		if (authHeader == null || !authHeader.startsWith("Bearer ")) {
			filterChain.doFilter(request, response);
			return;
		}

		String token = authHeader.substring(7);

		try {
			if (!jwtService.isTokenValid(token)) {
				filterChain.doFilter(request, response);
				return;
			}

			UUID userId = jwtService.extractUserId(token);
			User user = userRepository.findByIdAndDeletedAtIsNull(userId).orElse(null);

			if (user != null) {
				UsernamePasswordAuthenticationToken authentication =
					new UsernamePasswordAuthenticationToken(user, null, List.of());
				SecurityContextHolder.getContext().setAuthentication(authentication);
			}
		} catch (Exception ignored) {
			// Invalid or unreadable tokens fall back to anonymous access.
		}

		filterChain.doFilter(request, response);
	}
}
