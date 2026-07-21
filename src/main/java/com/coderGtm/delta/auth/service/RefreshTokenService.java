package com.coderGtm.delta.auth.service;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.security.SecureRandom;
import java.time.Instant;
import java.util.Base64;
import java.util.HexFormat;
import java.util.List;
import java.util.UUID;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import com.coderGtm.delta.auth.entity.RefreshToken;
import com.coderGtm.delta.auth.repository.RefreshTokenRepository;
import com.coderGtm.delta.common.exception.InvalidTokenException;
import com.coderGtm.delta.user.User;

import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;

@Service
@Slf4j
@RequiredArgsConstructor
public class RefreshTokenService {

	private static final int TOKEN_BYTE_LENGTH = 32;

	private final RefreshTokenRepository refreshTokenRepository;
	private final SecureRandom secureRandom = new SecureRandom();

	@Value("${jwt.refresh-token-expiration:2592000000}")
	private long refreshTokenExpiration;

	@Value("${jwt.refresh-token.revoked-retention:604800000}")
	private long revokedTokenRetention;

	@Transactional
	public IssuedRefreshToken create(User user) {
		String rawToken = generateRandomToken();

		RefreshToken refreshToken = new RefreshToken();
		refreshToken.setUser(user);
		refreshToken.setTokenHash(hashToken(rawToken));
		refreshToken.setExpiresAt(Instant.now().plusMillis(refreshTokenExpiration));
		refreshToken.setRevoked(false);

		refreshTokenRepository.save(refreshToken);

		return new IssuedRefreshToken(user, rawToken, refreshToken.getExpiresAt());
	}

	@Transactional(readOnly = true)
	public RefreshToken validate(String rawToken) {
		RefreshToken refreshToken = refreshTokenRepository
			.findByTokenHash(hashToken(rawToken))
			.orElseThrow(() -> new InvalidTokenException("Invalid refresh token"));

		if (refreshToken.isRevoked()) {
			throw new InvalidTokenException("Refresh token has been revoked");
		}

		if (refreshToken.getExpiresAt().isBefore(Instant.now())) {
			throw new InvalidTokenException("Refresh token has expired");
		}

		return refreshToken;
	}

	@Transactional
	public void revoke(String rawToken) {
		RefreshToken refreshToken = validate(rawToken);
		refreshToken.setRevoked(true);
		refreshTokenRepository.save(refreshToken);
	}

	@Transactional
	public IssuedRefreshToken rotate(String rawToken) {
		RefreshToken currentToken = validate(rawToken);

		currentToken.setRevoked(true);
		refreshTokenRepository.save(currentToken);

		return create(currentToken.getUser());
	}

	@Transactional
	public int revokeAllForUser(User user) {
		return revokeAllForUserId(user.getId());
	}

	@Transactional
	public int revokeAllForUserId(UUID userId) {
		List<RefreshToken> refreshTokens = refreshTokenRepository.findAllByUser_IdAndRevokedFalse(userId);

		refreshTokens.forEach(refreshToken -> refreshToken.setRevoked(true));
		refreshTokenRepository.saveAll(refreshTokens);

		return refreshTokens.size();
	}

	@Transactional
	@Scheduled(fixedDelayString = "${jwt.refresh-token.cleanup-interval:86400000}")
	public void cleanup() {
		long expiredDeleted = refreshTokenRepository.deleteByExpiresAtBefore(Instant.now());
		long revokedDeleted = refreshTokenRepository.deleteByRevokedTrueAndUpdatedAtBefore(
			Instant.now().minusMillis(revokedTokenRetention)
		);

		if (expiredDeleted > 0 || revokedDeleted > 0) {
			log.info(
				"Refresh token cleanup removed {} expired tokens and {} old revoked tokens",
				expiredDeleted,
				revokedDeleted
			);
		}
	}

	private String generateRandomToken() {
		byte[] bytes = new byte[TOKEN_BYTE_LENGTH];
		secureRandom.nextBytes(bytes);
		return Base64.getUrlEncoder().withoutPadding().encodeToString(bytes);
	}

	private String hashToken(String token) {
		try {
			MessageDigest digest = MessageDigest.getInstance("SHA-256");
			byte[] hash = digest.digest(token.getBytes(StandardCharsets.UTF_8));
			return HexFormat.of().formatHex(hash);
		} catch (NoSuchAlgorithmException e) {
			throw new IllegalStateException("SHA-256 algorithm is not available", e);
		}
	}

	public record IssuedRefreshToken(
		User user,
		String refreshToken,
		Instant expiresAt
	) {}
}
