package com.coderGtm.delta.auth;

import org.springframework.stereotype.Service;

import com.coderGtm.delta.auth.dto.FirebaseUserInfo;
import com.google.firebase.auth.FirebaseAuth;
import com.google.firebase.auth.FirebaseAuthException;
import com.google.firebase.auth.FirebaseToken;

import lombok.RequiredArgsConstructor;

@Service
@RequiredArgsConstructor
public class FirebaseService {

	private final FirebaseAuth firebaseAuth;

	public FirebaseUserInfo verifyIdToken(String idToken) throws FirebaseAuthException {

		FirebaseToken decodedToken = firebaseAuth.verifyIdToken(idToken);

		return new FirebaseUserInfo(
			decodedToken.getUid(),
			decodedToken.getName(),
			decodedToken.getEmail(),
			(String) decodedToken.getClaims().get("phone_number")
		);
	}
}
