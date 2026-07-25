package com.coderGtm.delta.outlet.dto;

import jakarta.validation.constraints.Email;
import jakarta.validation.constraints.NotBlank;

/**
 * Request payload used by an outlet owner to invite an employee via email.
 */
public record InviteOutletMemberRequest(
	@NotBlank(message = "Employee email is required")
	@Email(message = "Employee email must be a valid email address")
	String email
) {
}
