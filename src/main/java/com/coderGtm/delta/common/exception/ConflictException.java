package com.coderGtm.delta.common.exception;

import org.springframework.http.HttpStatus;

/**
 * Raised when a request conflicts with the current state of the resource.
 */
public class ConflictException extends ApiException {

	public ConflictException(String message) {
		super("CONFLICT", HttpStatus.CONFLICT, message);
	}

	public ConflictException(String message, Throwable cause) {
		super("CONFLICT", HttpStatus.CONFLICT, message, cause);
	}
}
