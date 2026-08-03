package com.coderGtm.delta.auth.config;

import org.springframework.boot.web.servlet.FilterRegistrationBean;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.security.config.annotation.web.builders.HttpSecurity;
import org.springframework.security.config.annotation.web.configuration.EnableWebSecurity;
import org.springframework.security.config.http.SessionCreationPolicy;
import org.springframework.security.web.SecurityFilterChain;
import org.springframework.security.web.authentication.UsernamePasswordAuthenticationFilter;

import com.coderGtm.delta.auth.filter.JwtAuthenticationFilter;
import com.coderGtm.delta.common.web.ApiPaths;
import com.coderGtm.delta.common.web.RateLimitingFilter;

import lombok.RequiredArgsConstructor;

/**
 * Configures stateless security for the API.
 *
 * <p>The application authenticates requests using a bearer JWT rather than an
 * HTTP session, so CSRF protection is disabled and the custom JWT filter is
 * inserted before Spring Security's username/password authentication filter.</p>
 */
@Configuration
@EnableWebSecurity
@RequiredArgsConstructor
public class SecurityConfig {

	private final JwtAuthenticationFilter jwtAuthenticationFilter;
	private final RateLimitingFilter rateLimitingFilter;

	/**
	 * Builds the application's security filter chain.
	 */
	@Bean
	public SecurityFilterChain securityFilterChain(HttpSecurity http) throws Exception {
		return http
			.csrf(csrf -> csrf.disable())
			.sessionManagement(sm -> sm.sessionCreationPolicy(SessionCreationPolicy.STATELESS))
			.authorizeHttpRequests(auth -> auth
				.requestMatchers(
					"/actuator/health",
					"/actuator/health/**",
					"/actuator/info",
					ApiPaths.AUTH + "/login",
					ApiPaths.AUTH + "/refresh",
					ApiPaths.AUTH + "/logout"
				).permitAll()
				.anyRequest().authenticated()
			)
			.addFilterBefore(jwtAuthenticationFilter, UsernamePasswordAuthenticationFilter.class)
			.addFilterAfter(rateLimitingFilter, JwtAuthenticationFilter.class)
			.build();
	}

	/**
	 * Prevents the rate-limiting filter from also being registered as a container
	 * filter outside Spring Security's authenticated filter chain.
	 */
	@Bean
	public FilterRegistrationBean<RateLimitingFilter> rateLimitingFilterRegistration(RateLimitingFilter filter) {
		FilterRegistrationBean<RateLimitingFilter> registration = new FilterRegistrationBean<>(filter);
		registration.setEnabled(false);
		return registration;
	}
}
