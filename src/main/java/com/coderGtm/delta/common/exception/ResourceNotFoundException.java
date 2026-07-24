package com.coderGtm.delta.common.exception;

import org.springframework.http.HttpStatus;

/**
 * Raised when the requested resource cannot be found.
 */
public class ResourceNotFoundException extends ApiException {

	public ResourceNotFoundException(String message) {
		super("NOT_FOUND", HttpStatus.NOT_FOUND, message);
	}
}
