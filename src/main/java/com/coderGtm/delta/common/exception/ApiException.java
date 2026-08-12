package com.coderGtm.delta.common.exception;

import org.springframework.http.HttpStatus;

import lombok.Getter;

/**
 * Base runtime exception used for business and API level failures that should be
 * surfaced to clients in a structured way.
 */
@Getter
public abstract class ApiException extends RuntimeException {

	private final String code;
	private final HttpStatus status;

	protected ApiException(String code, HttpStatus status, String message) {
		super(message);
		this.code = code;
		this.status = status;
	}

	protected ApiException(String code, HttpStatus status, String message, Throwable cause) {
		super(message, cause);
		this.code = code;
		this.status = status;
	}
}
