package com.coderGtm.delta.common.exception;

/**
 * Raised when an access or refresh token cannot be verified or used safely.
 */
public class InvalidTokenException extends RuntimeException {

	public InvalidTokenException(String message) {
		super(message);
	}

	public InvalidTokenException(String message, Throwable cause) {
		super(message, cause);
	}
}
