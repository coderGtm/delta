package com.coderGtm.delta.user;

import java.util.List;

import org.springframework.stereotype.Service;

import lombok.RequiredArgsConstructor;

/**
 * Handles user-related application operations.
 */
@Service
@RequiredArgsConstructor
public class UserService {

	private final UserRepository userRepository;

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
}
