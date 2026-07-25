package com.coderGtm.delta.auth.filter;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.atomic.AtomicBoolean;

import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.mock.web.MockHttpServletRequest;
import org.springframework.mock.web.MockHttpServletResponse;
import org.springframework.security.core.Authentication;
import org.springframework.security.core.context.SecurityContextHolder;

import com.coderGtm.delta.auth.service.JwtService;
import com.coderGtm.delta.user.User;
import com.coderGtm.delta.user.UserRepository;

import jakarta.servlet.FilterChain;

@ExtendWith(MockitoExtension.class)
class JwtAuthenticationFilterTest {

	@Mock
	private JwtService jwtService;

	@Mock
	private UserRepository userRepository;

	private JwtAuthenticationFilter filter;

	@BeforeEach
	void setUp() {
		SecurityContextHolder.clearContext();
		filter = new JwtAuthenticationFilter(jwtService, userRepository);
	}

	@AfterEach
	void tearDown() {
		SecurityContextHolder.clearContext();
	}

	@Test
	void noAuthorizationHeaderDoesNothingAndContinuesChain() throws Exception {
		MockHttpServletRequest request = new MockHttpServletRequest();
		MockHttpServletResponse response = new MockHttpServletResponse();
		AtomicBoolean chainCalled = new AtomicBoolean(false);
		FilterChain chain = (req, res) -> chainCalled.set(true);

		filter.doFilter(request, response, chain);

		assertThat(chainCalled).isTrue();
		assertThat(SecurityContextHolder.getContext().getAuthentication()).isNull();
		verifyNoInteractions(jwtService, userRepository);
	}

	@Test
	void nonBearerAuthorizationHeaderDoesNothingAndContinuesChain() throws Exception {
		MockHttpServletRequest request = new MockHttpServletRequest();
		request.addHeader("Authorization", "Basic abc123");
		MockHttpServletResponse response = new MockHttpServletResponse();
		AtomicBoolean chainCalled = new AtomicBoolean(false);
		FilterChain chain = (req, res) -> chainCalled.set(true);

		filter.doFilter(request, response, chain);

		assertThat(chainCalled).isTrue();
		assertThat(SecurityContextHolder.getContext().getAuthentication()).isNull();
		verifyNoInteractions(jwtService, userRepository);
	}

	@Test
	void invalidTokenDoesNotAuthenticateAndContinuesChain() throws Exception {
		MockHttpServletRequest request = new MockHttpServletRequest();
		request.addHeader("Authorization", "Bearer invalid-token");
		MockHttpServletResponse response = new MockHttpServletResponse();
		AtomicBoolean chainCalled = new AtomicBoolean(false);
		FilterChain chain = (req, res) -> chainCalled.set(true);

		when(jwtService.isTokenValid("invalid-token")).thenReturn(false);

		filter.doFilter(request, response, chain);

		assertThat(chainCalled).isTrue();
		assertThat(SecurityContextHolder.getContext().getAuthentication()).isNull();
		verify(jwtService).isTokenValid("invalid-token");
		verify(jwtService, never()).extractUserId("invalid-token");
		verifyNoInteractions(userRepository);
	}

	@Test
	void validTokenWithExistingUserAuthenticatesRequest() throws Exception {
		UUID userId = UUID.randomUUID();
		User user = new User();
		user.setId(userId);
		user.setName("Gautam");
		user.setEmail("gautam@example.com");

		MockHttpServletRequest request = new MockHttpServletRequest();
		request.addHeader("Authorization", "Bearer valid-token");
		MockHttpServletResponse response = new MockHttpServletResponse();
		AtomicBoolean chainCalled = new AtomicBoolean(false);
		FilterChain chain = (req, res) -> chainCalled.set(true);

		when(jwtService.isTokenValid("valid-token")).thenReturn(true);
		when(jwtService.extractUserId("valid-token")).thenReturn(userId);
		when(userRepository.findByIdAndDeletedAtIsNull(userId)).thenReturn(Optional.of(user));

		filter.doFilter(request, response, chain);

		Authentication authentication = SecurityContextHolder.getContext().getAuthentication();
		assertThat(chainCalled).isTrue();
		assertThat(authentication).isNotNull();
		assertThat(authentication.getPrincipal()).isEqualTo(user);
		assertThat(authentication.getCredentials()).isNull();
		assertThat(authentication.getAuthorities()).isEmpty();
	}

	@Test
	void validTokenWithMissingUserLeavesRequestUnauthenticated() throws Exception {
		UUID userId = UUID.randomUUID();
		MockHttpServletRequest request = new MockHttpServletRequest();
		request.addHeader("Authorization", "Bearer valid-token");
		MockHttpServletResponse response = new MockHttpServletResponse();
		AtomicBoolean chainCalled = new AtomicBoolean(false);
		FilterChain chain = (req, res) -> chainCalled.set(true);

		when(jwtService.isTokenValid("valid-token")).thenReturn(true);
		when(jwtService.extractUserId("valid-token")).thenReturn(userId);
		when(userRepository.findByIdAndDeletedAtIsNull(userId)).thenReturn(Optional.empty());

		filter.doFilter(request, response, chain);

		assertThat(chainCalled).isTrue();
		assertThat(SecurityContextHolder.getContext().getAuthentication()).isNull();
	}

	@Test
	void jwtExceptionsAreSwallowedAndChainStillContinues() throws Exception {
		MockHttpServletRequest request = new MockHttpServletRequest();
		request.addHeader("Authorization", "Bearer broken-token");
		MockHttpServletResponse response = new MockHttpServletResponse();
		AtomicBoolean chainCalled = new AtomicBoolean(false);
		FilterChain chain = (req, res) -> chainCalled.set(true);

		when(jwtService.isTokenValid("broken-token")).thenThrow(new RuntimeException("boom"));

		filter.doFilter(request, response, chain);

		assertThat(chainCalled).isTrue();
		assertThat(SecurityContextHolder.getContext().getAuthentication()).isNull();
		verifyNoInteractions(userRepository);
	}
}
