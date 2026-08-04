package com.coderGtm.delta.common.web;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.mock.web.MockHttpServletRequest;
import org.springframework.mock.web.MockHttpServletResponse;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.json.JsonMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;

import jakarta.servlet.FilterChain;

@ExtendWith(MockitoExtension.class)
class RateLimitingFilterTest {

	// Mirrors the Spring-managed ObjectMapper used by the app (jsr310 registered).
	private final ObjectMapper objectMapper = JsonMapper.builder().addModule(new JavaTimeModule()).build();

	private RateLimitingFilter filter;

	@BeforeEach
	void setUp() {
		filter = new RateLimitingFilter();
	}

	@Test
	void exceedingLoginBudgetReturnsWellFormed429() throws Exception {
		MockHttpServletRequest request = new MockHttpServletRequest("POST", "/api/v1/auth/login");
		request.setRemoteAddr("10.0.0.1");
		FilterChain chain = (req, res) -> {
		};

		for (int i = 0; i < 10; i++) {
			MockHttpServletResponse response = new MockHttpServletResponse();
			filter.doFilter(request, response, chain);
			assertThat(response.getStatus()).isEqualTo(200);
		}

		MockHttpServletResponse response = new MockHttpServletResponse();
		filter.doFilter(request, response, chain);

		assertThat(response.getStatus()).isEqualTo(429);
		assertThat(response.getHeader("Retry-After")).isNotNull();
		assertThat(response.getContentType()).startsWith("application/json");

		JsonNode body = objectMapper.readTree(response.getContentAsByteArray());
		assertThat(body.get("code").asText()).isEqualTo("RATE_LIMIT_EXCEEDED");
		assertThat(body.get("timestamp").isMissingNode()).isFalse();
	}

	@Test
	void differentClientIpsReceiveSeparateBudgets() throws Exception {
		FilterChain chain = (req, res) -> {
		};

		for (int i = 0; i < 11; i++) {
			MockHttpServletResponse response = new MockHttpServletResponse();
			filter.doFilter(loginRequest("10.0.0.1"), response, chain);
		}

		MockHttpServletResponse otherIp = new MockHttpServletResponse();
		filter.doFilter(loginRequest("10.0.0.2"), otherIp, chain);
		assertThat(otherIp.getStatus()).isEqualTo(200);
	}

	private MockHttpServletRequest loginRequest(String remoteAddr) {
		MockHttpServletRequest request = new MockHttpServletRequest("POST", "/api/v1/auth/login");
		request.setRemoteAddr(remoteAddr);
		return request;
	}
}
