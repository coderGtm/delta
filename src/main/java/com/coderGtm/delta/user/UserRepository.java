package com.coderGtm.delta.user;

import java.util.Optional;
import java.util.UUID;

import org.springframework.data.jpa.repository.JpaRepository;

public interface UserRepository extends JpaRepository<User, UUID> {
	
	Optional<User> findByAuthUid(String authUid);
}
