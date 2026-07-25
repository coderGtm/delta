package com.coderGtm.delta.auth.repository;

import java.time.Instant;
import java.util.List;
import java.util.Optional;
import java.util.UUID;

import org.springframework.data.jpa.repository.JpaRepository;

import com.coderGtm.delta.auth.entity.RefreshToken;

/**
 * Repository for refresh token persistence and maintenance queries.
 */
public interface RefreshTokenRepository extends JpaRepository<RefreshToken, UUID> {

	Optional<RefreshToken> findByTokenHash(String tokenHash);

	List<RefreshToken> findAllByUser_IdAndRevokedFalse(UUID userId);

	long deleteByExpiresAtBefore(Instant cutoff);

	long deleteByRevokedTrueAndUpdatedAtBefore(Instant cutoff);
}
