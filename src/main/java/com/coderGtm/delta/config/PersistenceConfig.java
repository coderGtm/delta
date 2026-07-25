package com.coderGtm.delta.config;

import java.time.Clock;
import java.time.Instant;
import java.util.Optional;
import java.util.UUID;

import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.data.auditing.DateTimeProvider;
import org.springframework.data.domain.AuditorAware;
import org.springframework.security.core.Authentication;
import org.springframework.security.core.context.SecurityContextHolder;

import com.coderGtm.delta.user.User;

/**
 * Provides shared persistence infrastructure such as UTC time and JPA auditing
 * support for user-based audit fields.
 */
@Configuration
public class PersistenceConfig {

	/**
	 * Exposes a shared UTC clock so business services and auditing can use a
	 * deterministic time source in tests.
	 */
	@Bean
	public Clock clock() {
		return Clock.systemUTC();
	}

	/**
	 * Resolves the authenticated application's user identifier for JPA
	 * {@code createdBy} and {@code updatedBy} fields.
	 */
	@Bean
	public AuditorAware<UUID> auditorAware() {
		return () -> Optional.ofNullable(SecurityContextHolder.getContext().getAuthentication())
			.map(Authentication::getPrincipal)
			.filter(User.class::isInstance)
			.map(User.class::cast)
			.map(User::getId);
	}

	/**
	 * Makes Spring Data auditing timestamps use the shared application clock.
	 */
	@Bean
	public DateTimeProvider auditingDateTimeProvider(Clock clock) {
		return () -> Optional.of(Instant.now(clock));
	}
}
