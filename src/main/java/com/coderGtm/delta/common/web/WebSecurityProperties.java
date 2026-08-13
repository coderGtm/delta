package com.coderGtm.delta.common.web;

import org.springframework.boot.context.properties.ConfigurationProperties;

/**
 * Controls whether the application trusts client-supplied proxy headers when
 * resolving the remote IP address.
 *
 * <p>Must be enabled only when the service sits behind a trusted reverse proxy
 * or gateway that overwrites {@code X-Forwarded-For} on every request. When set
 * to {@code false}, IP resolution relies solely on the socket remote address,
 * preventing an attacker from spoofing IP-based rate-limit keys by sending a
 * fabricated {@code X-Forwarded-For} header.</p>
 */
@ConfigurationProperties(prefix = "app.web")
public record WebSecurityProperties(
	boolean trustProxyHeaders
) {
}