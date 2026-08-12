package com.coderGtm.delta.user;

import org.springframework.http.ResponseEntity;
import org.springframework.security.core.annotation.AuthenticationPrincipal;
import org.springframework.validation.annotation.Validated;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import com.coderGtm.delta.common.web.ApiPaths;

import lombok.RequiredArgsConstructor;

/**
 * REST endpoints for the authenticated user's own account.
 */
@RestController
@RequestMapping(ApiPaths.USERS)
@RequiredArgsConstructor
@Validated
public class UserController {

	private final UserService userService;

	/**
	 * Permanently closes the authenticated user's account. The account is
	 * deleted from the authentication provider and soft-deleted locally, keeping
	 * historical attendance and membership records intact.
	 */
	@DeleteMapping("/me")
	public ResponseEntity<Void> deleteAccount(@AuthenticationPrincipal User currentUser) {
		userService.deleteAccount(currentUser);
		return ResponseEntity.noContent().build();
	}
}
