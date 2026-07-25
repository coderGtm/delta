package com.coderGtm.delta;

import static org.mockito.Mockito.mock;

import org.springframework.boot.test.context.TestConfiguration;
import org.springframework.context.annotation.Bean;

import com.google.firebase.FirebaseApp;
import com.google.firebase.auth.FirebaseAuth;

@TestConfiguration(proxyBeanMethods = false)
public class TestFirebaseConfiguration {

	@Bean("firebaseApp")
	FirebaseApp firebaseApp() {
		return mock(FirebaseApp.class);
	}

	@Bean("firebaseAuth")
	FirebaseAuth firebaseAuth() {
		return mock(FirebaseAuth.class);
	}
}
