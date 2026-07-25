package com.coderGtm.delta.auth.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.sql.Timestamp;
import java.time.Instant;
import java.util.Comparator;
import java.util.HexFormat;
import java.util.List;
import java.util.Set;
import java.util.stream.Collectors;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.transaction.annotation.Transactional;

import com.coderGtm.delta.TestApplication;
import com.coderGtm.delta.TestFirebaseConfiguration;
import com.coderGtm.delta.auth.entity.RefreshToken;
import com.coderGtm.delta.auth.repository.RefreshTokenRepository;
import com.coderGtm.delta.common.exception.InvalidTokenException;
import com.coderGtm.delta.user.User;
import com.coderGtm.delta.user.UserRepository;

@SpringBootTest(
	classes = { TestApplication.class, TestFirebaseConfiguration.class },
	properties = {
		"jwt.refresh-token-expiration=60000",
		"jwt.refresh-token.revoked-retention=1000"
	}
)
@Transactional
class RefreshTokenServiceIntegrationTest {

	@Autowired
	private RefreshTokenService refreshTokenService;

	@Autowired
	private RefreshTokenRepository refreshTokenRepository;

	@Autowired
	private UserRepository userRepository;

	@Autowired
	private JdbcTemplate jdbcTemplate;

	@Test
	void createPersistsHashedTokenAndReturnsRawToken() {
		User user = persistUser("create-user");
		Instant before = Instant.now();

		RefreshTokenService.IssuedRefreshToken issued = refreshTokenService.create(user);

		List<RefreshToken> storedTokens = refreshTokenRepository.findAll();
		assertThat(storedTokens).hasSize(1);

		RefreshToken stored = storedTokens.getFirst();
		assertThat(issued.refreshToken()).isNotBlank();
		assertThat(stored.getTokenHash()).isNotEqualTo(issued.refreshToken());
		assertThat(stored.getTokenHash()).isEqualTo(sha256Hex(issued.refreshToken()));
		assertThat(stored.getUser().getId()).isEqualTo(user.getId());
		assertThat(stored.isRevoked()).isFalse();
		assertThat(stored.getExpiresAt()).isAfter(before.plusSeconds(50));
		assertThat(stored.getExpiresAt()).isBefore(before.plusSeconds(70));
	}

	@Test
	void validateReturnsActiveToken() {
		User user = persistUser("validate-user");
		RefreshTokenService.IssuedRefreshToken issued = refreshTokenService.create(user);

		RefreshToken refreshToken = refreshTokenService.validate(issued.refreshToken());

		assertThat(refreshToken.getUser().getId()).isEqualTo(user.getId());
		assertThat(refreshToken.isRevoked()).isFalse();
	}

	@Test
	void validateThrowsWhenTokenDoesNotExist() {
		assertThatThrownBy(() -> refreshTokenService.validate("missing-token"))
			.isInstanceOf(InvalidTokenException.class)
			.hasMessage("Invalid refresh token");
	}

	@Test
	void validateThrowsWhenTokenIsRevoked() {
		User user = persistUser("revoked-user");
		RefreshTokenService.IssuedRefreshToken issued = refreshTokenService.create(user);
		RefreshToken refreshToken = refreshTokenRepository.findAll().getFirst();
		refreshToken.setRevoked(true);
		refreshTokenRepository.saveAndFlush(refreshToken);

		assertThatThrownBy(() -> refreshTokenService.validate(issued.refreshToken()))
			.isInstanceOf(InvalidTokenException.class)
			.hasMessage("Refresh token has been revoked");
	}

	@Test
	void validateThrowsWhenTokenIsExpired() {
		User user = persistUser("expired-user");
		RefreshTokenService.IssuedRefreshToken issued = refreshTokenService.create(user);
		RefreshToken refreshToken = refreshTokenRepository.findAll().getFirst();
		refreshToken.setExpiresAt(Instant.now().minusSeconds(1));
		refreshTokenRepository.saveAndFlush(refreshToken);

		assertThatThrownBy(() -> refreshTokenService.validate(issued.refreshToken()))
			.isInstanceOf(InvalidTokenException.class)
			.hasMessage("Refresh token has expired");
	}

	@Test
	void revokeMarksTokenAsRevoked() {
		User user = persistUser("revoke-user");
		RefreshTokenService.IssuedRefreshToken issued = refreshTokenService.create(user);

		refreshTokenService.revoke(issued.refreshToken());

		RefreshToken refreshToken = refreshTokenRepository.findAll().getFirst();
		assertThat(refreshToken.isRevoked()).isTrue();
	}

	@Test
	void rotateRevokesOldTokenAndCreatesNewToken() {
		User user = persistUser("rotate-user");
		RefreshTokenService.IssuedRefreshToken issued = refreshTokenService.create(user);

		RefreshTokenService.IssuedRefreshToken rotated = refreshTokenService.rotate(issued.refreshToken());

		assertThat(rotated.refreshToken()).isNotEqualTo(issued.refreshToken());
		assertThat(rotated.user().getId()).isEqualTo(user.getId());
		assertThat(refreshTokenRepository.findAll()).hasSize(2);

		List<RefreshToken> tokens = refreshTokenRepository.findAll().stream()
			.sorted(Comparator.comparing(RefreshToken::getCreatedAt, Comparator.nullsLast(Comparator.naturalOrder())))
			.toList();
		assertThat(tokens.getFirst().isRevoked()).isTrue();
		assertThat(tokens.getLast().isRevoked()).isFalse();
		assertThatThrownBy(() -> refreshTokenService.validate(issued.refreshToken()))
			.isInstanceOf(InvalidTokenException.class);
		assertThat(refreshTokenService.validate(rotated.refreshToken()).getUser().getId()).isEqualTo(user.getId());
	}

	@Test
	void revokeAllForUserIdRevokesOnlyThatUsersActiveTokens() {
		User firstUser = persistUser("first-user");
		User secondUser = persistUser("second-user");
		refreshTokenService.create(firstUser);
		refreshTokenService.create(firstUser);
		refreshTokenService.create(secondUser);

		int revokedCount = refreshTokenService.revokeAllForUserId(firstUser.getId());

		assertThat(revokedCount).isEqualTo(2);
		List<RefreshToken> firstUserTokens = refreshTokenRepository.findAll().stream()
			.filter(refreshToken -> refreshToken.getUser().getId().equals(firstUser.getId()))
			.toList();
		List<RefreshToken> secondUserTokens = refreshTokenRepository.findAll().stream()
			.filter(refreshToken -> refreshToken.getUser().getId().equals(secondUser.getId()))
			.toList();
		assertThat(firstUserTokens).allMatch(RefreshToken::isRevoked);
		assertThat(secondUserTokens).allMatch(refreshToken -> !refreshToken.isRevoked());
	}

	@Test
	void cleanupDeletesExpiredTokensAndOldRevokedTokens() {
		User user = persistUser("cleanup-user");
		Instant now = Instant.now();

		RefreshToken active = persistToken(user, "active", now.plusSeconds(3600), false, now);
		persistToken(user, "expired", now.minusSeconds(10), false, now.minusSeconds(10));
		persistToken(user, "revoked-old", now.plusSeconds(3600), true, now.minusSeconds(10));
		RefreshToken revokedRecent = persistToken(user, "revoked-recent", now.plusSeconds(3600), true, now);

		refreshTokenService.cleanup();

		List<RefreshToken> remaining = refreshTokenRepository.findAll();
		Set<String> remainingHashes = remaining.stream()
			.map(RefreshToken::getTokenHash)
			.collect(Collectors.toSet());

		assertThat(remaining).hasSize(2);
		assertThat(remainingHashes).containsExactlyInAnyOrder(active.getTokenHash(), revokedRecent.getTokenHash());
	}

	private User persistUser(String authUidPrefix) {
		User user = new User();
		user.setAuthUid(authUidPrefix + "-uid");
		user.setName("Test User");
		user.setEmail(authUidPrefix + "@example.com");
		user.setPhone("+911234567890");
		return userRepository.saveAndFlush(user);
	}

	private RefreshToken persistToken(User user, String rawToken, Instant expiresAt, boolean revoked, Instant updatedAt) {
		RefreshToken refreshToken = new RefreshToken();
		refreshToken.setUser(user);
		refreshToken.setTokenHash(sha256Hex(rawToken));
		refreshToken.setExpiresAt(expiresAt);
		refreshToken.setRevoked(revoked);
		RefreshToken saved = refreshTokenRepository.saveAndFlush(refreshToken);
		jdbcTemplate.update(
			"update refresh_tokens set created_at = ?, updated_at = ? where id = ?",
			Timestamp.from(updatedAt),
			Timestamp.from(updatedAt),
			saved.getId()
		);
		return refreshTokenRepository.findById(saved.getId()).orElseThrow();
	}

	private String sha256Hex(String value) {
		try {
			MessageDigest digest = MessageDigest.getInstance("SHA-256");
			return HexFormat.of().formatHex(digest.digest(value.getBytes(StandardCharsets.UTF_8)));
		} catch (NoSuchAlgorithmException e) {
			throw new IllegalStateException(e);
		}
	}
}
