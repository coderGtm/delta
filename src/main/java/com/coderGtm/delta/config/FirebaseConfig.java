package com.coderGtm.delta.config;

import java.io.FileInputStream;
import java.io.IOException;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

import com.google.auth.oauth2.GoogleCredentials;
import com.google.firebase.FirebaseApp;
import com.google.firebase.FirebaseOptions;
import com.google.firebase.auth.FirebaseAuth;

@Configuration
public class FirebaseConfig {

	@Value("${firebase.service-account}")
	private String serviceAccountPath;
	
	@Bean
	FirebaseApp firebaseApp() throws IOException {

		FileInputStream serviceAccount = new FileInputStream(serviceAccountPath);

		FirebaseOptions options = FirebaseOptions.builder()
			.setCredentials(GoogleCredentials.fromStream(serviceAccount))
			.build();

		if (FirebaseApp.getApps().isEmpty()) {
			return FirebaseApp.initializeApp(options);
		}

		return FirebaseApp.getInstance();
	}

	@Bean
	FirebaseAuth firebaseAuth(FirebaseApp firebaseApp) {
		return FirebaseAuth.getInstance(firebaseApp);
	}
}
