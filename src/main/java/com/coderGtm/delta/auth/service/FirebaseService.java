package com.coderGtm.delta.auth.service;

import org.springframework.stereotype.Service;

import com.coderGtm.delta.auth.dto.FirebaseUserInfo;
import com.google.firebase.auth.FirebaseAuth;
import com.google.firebase.auth.FirebaseAuthException;
import com.google.firebase.auth.FirebaseToken;

import lombok.RequiredArgsConstructor;

/**
 * Wraps Firebase Admin SDK interactions used by the authentication layer.
 */
@Service
@RequiredArgsConstructor
public class FirebaseService {

	private final FirebaseAuth firebaseAuth;

	/**
	 * Verifies a Firebase ID token and extracts only the user fields needed by
	 * the local application.
	 */
	public FirebaseUserInfo verifyIdToken(String idToken) throws FirebaseAuthException {
		FirebaseToken decodedToken = firebaseAuth.verifyIdToken(idToken);

		return new FirebaseUserInfo(
			decodedToken.getUid(),
			decodedToken.getName(),
			decodedToken.getEmail(),
			(String) decodedToken.getClaims().get("phone_number")
		);
	}

	/**
	 * Deletes the Firebase Auth record for the given UID. A later sign-in with
	 * the same provider account receives a fresh UID, allowing a completely new
	 * local account to be created without ties to the old one.
	 */
	public void deleteUser(String authUid) throws FirebaseAuthException {
		firebaseAuth.deleteUser(authUid);
	}
}
