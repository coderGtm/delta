package com.coderGtm.delta.common.exception;

import org.springframework.http.HttpStatus;

/**
 * Raised when a request is syntactically valid but violates a business rule.
 */
public class BadRequestException extends ApiException {

	public BadRequestException(String message) {
		super("BAD_REQUEST", HttpStatus.BAD_REQUEST, message);
	}
}
