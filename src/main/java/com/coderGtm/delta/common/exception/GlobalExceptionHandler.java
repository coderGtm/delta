package com.coderGtm.delta.common.exception;

import java.time.Instant;
import java.util.stream.Collectors;

import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.validation.FieldError;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;

import com.coderGtm.delta.common.dto.ErrorResponse;

/**
 * Translates application exceptions into stable JSON error responses.
 */
@RestControllerAdvice
public class GlobalExceptionHandler {
	
	@ExceptionHandler(InvalidTokenException.class)
	public ResponseEntity<ErrorResponse> handleInvalidToken(InvalidTokenException ex) {
		return buildResponse(HttpStatus.UNAUTHORIZED, "INVALID_TOKEN", ex.getMessage());
	}

	@ExceptionHandler(ApiException.class)
	public ResponseEntity<ErrorResponse> handleApiException(ApiException ex) {
		return buildResponse(ex.getStatus(), ex.getCode(), ex.getMessage());
	}

	@ExceptionHandler(MethodArgumentNotValidException.class)
	public ResponseEntity<ErrorResponse> handleValidationFailure(MethodArgumentNotValidException ex) {
		String message = ex.getBindingResult().getFieldErrors().stream()
			.map(FieldError::getDefaultMessage)
			.collect(Collectors.joining(", "));

		return buildResponse(HttpStatus.BAD_REQUEST, "VALIDATION_ERROR", message);
	}

	private ResponseEntity<ErrorResponse> buildResponse(HttpStatus status, String code, String message) {
		ErrorResponse response = new ErrorResponse(code, message, Instant.now());
		return ResponseEntity.status(status).body(response);
	}
}
