package com.coderGtm.delta.outlet.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

/**
 * Request payload used by an outlet owner to set a member's display name.
 */
public record UpdateMembershipDisplayNameRequest(
	@NotBlank(message = "Display name is required")
	@Size(max = 255, message = "Display name must be at most 255 characters")
	String displayName
) {
}