package com.coderGtm.delta.auth.config;

import org.springframework.boot.web.servlet.FilterRegistrationBean;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.security.config.annotation.web.builders.HttpSecurity;
import org.springframework.security.config.annotation.web.configuration.EnableWebSecurity;
import org.springframework.security.config.http.SessionCreationPolicy;
import org.springframework.security.web.SecurityFilterChain;
import org.springframework.security.web.authentication.UsernamePasswordAuthenticationFilter;

import com.coderGtm.delta.auth.filter.JwtAuthenticationFilter;
import com.coderGtm.delta.common.monitoring.PrometheusAccessProperties;
import com.coderGtm.delta.common.web.ApiPaths;
import com.coderGtm.delta.common.web.RateLimitingFilter;
import com.coderGtm.delta.common.web.WebSecurityProperties;

import jakarta.servlet.http.HttpServletRequest;
import lombok.RequiredArgsConstructor;
import org.springframework.security.authorization.AuthorizationDecision;
import org.springframework.security.web.header.writers.ReferrerPolicyHeaderWriter;

/**
 * Configures stateless security for the API.
 *
 * <p>The application authenticates requests using a bearer JWT rather than an
 * HTTP session, so CSRF protection is disabled and the custom JWT filter is
 * inserted before Spring Security's username/password authentication filter.</p>
 */
@Configuration
@EnableWebSecurity
@EnableConfigurationProperties({PrometheusAccessProperties.class, WebSecurityProperties.class})
@RequiredArgsConstructor
public class SecurityConfig {

	private final JwtAuthenticationFilter jwtAuthenticationFilter;
	private final RateLimitingFilter rateLimitingFilter;
	private final PrometheusAccessProperties prometheusAccessProperties;

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
					"/docs/**",
					"/swagger-ui/**",
					"/webjars/**",
					ApiPaths.AUTH + "/login",
					ApiPaths.AUTH + "/refresh",
					ApiPaths.AUTH + "/logout"
				).permitAll()
				.requestMatchers("/actuator/prometheus").access((authentication, context) ->
					new AuthorizationDecision(hasValidPrometheusToken(context.getRequest()))
				)
				.anyRequest().authenticated()
			)
			.addFilterBefore(jwtAuthenticationFilter, UsernamePasswordAuthenticationFilter.class)
			.addFilterAfter(rateLimitingFilter, JwtAuthenticationFilter.class)
			.headers(headers -> headers
				.frameOptions(frameOptions -> frameOptions.deny())
				.referrerPolicy(referrer -> referrer.policy(ReferrerPolicyHeaderWriter.ReferrerPolicy.NO_REFERRER))
				.permissionsPolicyHeader(permissions -> permissions.policy(
					"camera=(), microphone=(), geolocation=(), payment=(), usb=()"
				))
				.contentSecurityPolicy(csp -> csp.policyDirectives(
					"default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; "
						+ "style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; "
						+ "connect-src 'self'; frame-ancestors 'none'"
				))
				.httpStrictTransportSecurity(hsts -> hsts.includeSubDomains(true).maxAgeInSeconds(31536000))
			)
			.build();
	}

	/**
	 * Prevents the rate-limiting filter from also being registered as a container
	 * filter outside Spring Security's authenticated filter chain.
	 */
	private boolean hasValidPrometheusToken(HttpServletRequest request) {
		String authorization = request.getHeader("Authorization");
		if (authorization == null || !authorization.startsWith("Bearer ")) {
			return false;
		}
		return prometheusAccessProperties.matches(authorization.substring(7));
	}

	@Bean
	public FilterRegistrationBean<RateLimitingFilter> rateLimitingFilterRegistration(RateLimitingFilter filter) {
		FilterRegistrationBean<RateLimitingFilter> registration = new FilterRegistrationBean<>(filter);
		registration.setEnabled(false);
		return registration;
	}
}
