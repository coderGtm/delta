package com.coderGtm.delta.user;

import java.util.List;

import org.springframework.stereotype.Service;

import lombok.RequiredArgsConstructor;

@Service
@RequiredArgsConstructor
public class UserService {
	
	private final UserRepository userRepository;

	public List<User> getUsers() {
		return userRepository.findAll();
	}

	public User createUser(String authUid, String name, String email, String phoneNumber) {

		User user = new User();

		user.setAuthUid(authUid);
		user.setName(name);
		user.setEmail(email);
		user.setPhone(phoneNumber);

		return userRepository.save(user);
	}
}
