package com.coderGtm.delta.common.monitoring;

import org.springframework.boot.context.properties.ConfigurationProperties;

/**
 * Security settings for Prometheus scraping of actuator metrics.
 */
@ConfigurationProperties(prefix = "app.monitoring.prometheus")
public record PrometheusAccessProperties(
	String token
) {
	/**
	 * Returns whether the configured scrape token matches a bearer token value.
	 */
	public boolean matches(String bearerToken) {
		return token != null && !token.isBlank() && token.equals(bearerToken);
	}
}
