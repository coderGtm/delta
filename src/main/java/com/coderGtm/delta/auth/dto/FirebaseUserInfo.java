package com.coderGtm.delta.auth.dto;

public record FirebaseUserInfo(
	String uid,
	String name,
	String email,
	String phoneNumber
) {}
