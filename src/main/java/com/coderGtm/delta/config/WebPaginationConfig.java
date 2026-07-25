package com.coderGtm.delta.config;

import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.data.domain.PageRequest;
import org.springframework.data.web.config.PageableHandlerMethodArgumentResolverCustomizer;

/**
 * Centralizes pageable web defaults for collection endpoints.
 */
@Configuration
public class WebPaginationConfig {

	private static final int DEFAULT_PAGE_SIZE = 20;
	private static final int MAX_PAGE_SIZE = 100;

	/**
	 * Applies consistent page parameter behavior across the application.
	 */
	@Bean
	public PageableHandlerMethodArgumentResolverCustomizer pageableCustomizer() {
		return resolver -> {
			resolver.setOneIndexedParameters(false);
			resolver.setMaxPageSize(MAX_PAGE_SIZE);
			resolver.setFallbackPageable(PageRequest.of(0, DEFAULT_PAGE_SIZE));
		};
	}
}
