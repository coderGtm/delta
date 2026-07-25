package com.coderGtm.delta.outlet.dto;

import java.time.Instant;
import java.util.UUID;

import com.coderGtm.delta.outlet.entity.OutletMembershipStatus;
import com.coderGtm.delta.outlet.entity.OutletRole;

/**
 * API representation of a user's membership in an outlet.
 */
public record OutletMembershipResponse(
	UUID membershipId,
	OutletResponse outlet,
	UUID userId,
	String userName,
	String userEmail,
	OutletRole role,
	OutletMembershipStatus status,
	UUID invitedByUserId,
	String invitedByUserName,
	Instant createdAt,
	Instant updatedAt
) {
}
