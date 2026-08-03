package com.coderGtm.delta.common.health;

import org.springframework.boot.health.contributor.Health;
import org.springframework.boot.health.contributor.HealthIndicator;
import org.springframework.stereotype.Component;

import com.google.firebase.auth.FirebaseAuth;

import lombok.RequiredArgsConstructor;

/**
 * Lightweight Firebase health indicator that verifies the Firebase SDK beans
 * are initialized and available to the application.
 */
@Component("firebase")
@RequiredArgsConstructor
public class FirebaseHealthIndicator implements HealthIndicator {

	private final FirebaseAuth firebaseAuth;

	@Override
	public Health health() {
		try {
			firebaseAuth.hashCode();
			return Health.up().build();
		} catch (Exception ex) {
			return Health.down(ex).build();
		}
	}
}
