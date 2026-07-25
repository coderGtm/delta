package com.coderGtm.delta.user;

import java.util.Optional;
import java.util.UUID;

import org.springframework.data.jpa.repository.JpaRepository;

/**
 * Repository for querying local application users.
 */
public interface UserRepository extends JpaRepository<User, UUID> {

	Optional<User> findByAuthUid(String authUid);

	Optional<User> findByIdAndDeletedAtIsNull(UUID id);

	Optional<User> findByEmailIgnoreCaseAndDeletedAtIsNull(String email);
}
