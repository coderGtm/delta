package com.coderGtm.delta.auth.controller;

import org.springframework.http.ResponseEntity;
import org.springframework.security.core.annotation.AuthenticationPrincipal;
import org.springframework.validation.annotation.Validated;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import com.coderGtm.delta.auth.dto.LoginRequest;
import com.coderGtm.delta.auth.dto.LoginResponse;
import com.coderGtm.delta.auth.dto.LogoutRequest;
import com.coderGtm.delta.auth.dto.RefreshTokenRequest;
import com.coderGtm.delta.auth.dto.RefreshTokenResponse;
import com.coderGtm.delta.auth.service.AuthService;
import com.coderGtm.delta.user.User;

import jakarta.validation.Valid;
import lombok.RequiredArgsConstructor;

@RestController
@RequestMapping("/auth")
@RequiredArgsConstructor
@Validated
public class AuthController {

	private final AuthService authService;

	@PostMapping("/login")
	public ResponseEntity<LoginResponse> login(@Valid @RequestBody LoginRequest request) {
		return ResponseEntity.ok(authService.login(request));
	}

	@PostMapping("/refresh")
	public ResponseEntity<RefreshTokenResponse> refresh(@Valid @RequestBody RefreshTokenRequest request) {
		return ResponseEntity.ok(authService.refresh(request));
	}

	@PostMapping("/logout")
	public ResponseEntity<Void> logout(@Valid @RequestBody LogoutRequest request) {
		authService.logout(request);
		return ResponseEntity.noContent().build();
	}

	@PostMapping("/logout-all")
	public ResponseEntity<Void> logoutAll(@AuthenticationPrincipal User user) {
		authService.logoutAll(user);
		return ResponseEntity.noContent().build();
	}
}
