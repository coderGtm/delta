package com.coderGtm.delta.common.exception;

import java.time.Instant;

import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;

import com.coderGtm.delta.common.dto.ErrorResponse;

@RestControllerAdvice
public class GlobalExceptionHandler {
	
	@ExceptionHandler(InvalidTokenException.class)
	public ResponseEntity<ErrorResponse> handleInvalidToken(
		InvalidTokenException ex
	) {

		ErrorResponse response = new ErrorResponse(
			"INVALID_TOKEN", 
			ex.getMessage(),
			Instant.now()
		);

		return ResponseEntity.status(HttpStatus.UNAUTHORIZED)
			.body(response);
	}
}
