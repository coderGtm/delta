package com.coderGtm.delta.common.exception;

import org.springframework.http.HttpStatus;

/**
 * Raised when the authenticated user is not allowed to perform an action.
 */
public class ForbiddenException extends ApiException {

	public ForbiddenException(String message) {
		super("FORBIDDEN", HttpStatus.FORBIDDEN, message);
	}
}
