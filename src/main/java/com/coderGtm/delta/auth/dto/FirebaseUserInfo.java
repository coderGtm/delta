package com.coderGtm.delta.auth.dto;

/**
 * Small projection of Firebase user details needed to hydrate the local user
 * record.
 */
public record FirebaseUserInfo(
	String uid,
	String name,
	String email,
	String phoneNumber
) {
}
