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

@Component
@RequiredArgsConstructor
public class JwtAuthenticationFilter extends OncePerRequestFilter {
	
	private final JwtService jwtService;
	private final UserRepository userRepository;

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

					User user = userRepository.findById(userId).orElse(null);

					if (user != null) {

						UsernamePasswordAuthenticationToken auth = 
							new UsernamePasswordAuthenticationToken(
								user, 
								null, 
								List.of()
							);

						SecurityContextHolder.getContext().setAuthentication(auth);
					}
				} catch (Exception ignored) {
					// silently fail -> request will be ignored
				}

				filterChain.doFilter(request, response);
			}
}
