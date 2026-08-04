package com.coderGtm.delta.config;

import io.swagger.v3.oas.models.Components;
import io.swagger.v3.oas.models.OpenAPI;
import io.swagger.v3.oas.models.info.Info;
import io.swagger.v3.oas.models.security.SecurityRequirement;
import io.swagger.v3.oas.models.security.SecurityScheme;

import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * Configures the runtime-generated OpenAPI document served by springdoc.
 *
 * <p>The document is derived from controllers and DTOs, so it stays in sync
 * with the codebase without a hand-maintained spec file.</p>
 */
@Configuration
public class OpenApiConfig {

	private static final String BEARER_AUTH = "bearerAuth";

	/**
	 * Builds the OpenAPI document metadata and the shared JWT bearer security
	 * scheme used by protected endpoints.
	 */
	@Bean
	public OpenAPI deltaOpenAPI() {
		return new OpenAPI()
			.info(new Info()
				.title("Delta API")
				.description("Employee attendance backend API.")
				.version("v1"))
			.components(new Components()
				.addSecuritySchemes(BEARER_AUTH,
					new SecurityScheme()
						.name(BEARER_AUTH)
						.type(SecurityScheme.Type.HTTP)
						.scheme("bearer")
						.bearerFormat("JWT")))
			.addSecurityItem(new SecurityRequirement().addList(BEARER_AUTH));
	}
}
