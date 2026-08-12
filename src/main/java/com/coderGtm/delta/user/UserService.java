package com.coderGtm.delta.user;

import java.time.Instant;
import java.util.List;
import java.util.Map;

import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import com.coderGtm.delta.auth.service.FirebaseService;
import com.coderGtm.delta.auth.service.RefreshTokenService;
import com.coderGtm.delta.common.audit.service.AuditService;
import com.coderGtm.delta.common.exception.ConflictException;
import com.coderGtm.delta.common.metrics.ApplicationMetrics;
import com.google.firebase.auth.FirebaseAuthException;

import lombok.RequiredArgsConstructor;

/**
 * Handles user-related application operations.
 */
@Service
@RequiredArgsConstructor
public class UserService {

	private final UserRepository userRepository;
	private final FirebaseService firebaseService;
	private final RefreshTokenService refreshTokenService;
	private final AuditService auditService;
	private final ApplicationMetrics applicationMetrics;

	/**
	 * Returns all persisted users.
	 */
	public List<User> getUsers() {
		return userRepository.findAll();
	}

	/**
	 * Creates a new local user record based on authentication-provider details.
	 */
	public User createUser(String authUid, String name, String email, String phoneNumber) {
		User user = new User();
		user.setAuthUid(authUid);
		user.setName(name);
		user.setEmail(email);
		user.setPhone(phoneNumber);

		return userRepository.save(user);
	}

	/**
	 * Permanently closes an account.
	 *
	 * <p>The Firebase Auth record is deleted first so that a future sign-in with
	 * the same provider account receives a fresh UID. The local record is then
	 * soft-deleted, all refresh tokens are revoked, and the email is moved to
	 * {@code historicalEmail} so the unique email constraint no longer blocks a
	 * brand-new account with the same email.</p>
	 */
	@Transactional
	public void deleteAccount(User user) {
		if (user.getDeletedAt() != null) {
			throw new ConflictException("Account has already been deleted");
		}

		try {
			firebaseService.deleteUser(user.getAuthUid());
		} catch (FirebaseAuthException e) {
			throw new ConflictException("Failed to delete the user from the authentication provider", e);
		}

		refreshTokenService.revokeAllForUserId(user.getId());

		user.setHistoricalEmail(user.getEmail());
		user.setEmail(null);
		user.setDeletedAt(Instant.now());

		userRepository.save(user);
		applicationMetrics.increment("user.deleted");
		auditService.record(
			user.getId(),
			"USER_DELETED",
			"USER",
			user.getId(),
			Map.of("historicalEmail", String.valueOf(user.getHistoricalEmail()))
		);
	}
}
