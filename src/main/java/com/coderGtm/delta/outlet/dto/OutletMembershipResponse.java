package com.coderGtm.delta.outlet.dto;

import java.time.Instant;
import java.util.UUID;

import com.coderGtm.delta.outlet.entity.OutletMembershipStatus;
import com.coderGtm.delta.outlet.entity.OutletRole;

/**
 * API representation of a user's membership in an outlet.
 *
 * <p>{@code displayName} is the owner-controlled name to render for this member
 * in frontend lists; it falls back to the user's account name when not set.</p>
 */
public record OutletMembershipResponse(
	UUID membershipId,
	OutletResponse outlet,
	UUID userId,
	String userName,
	String userEmail,
	String displayName,
	OutletRole role,
	OutletMembershipStatus status,
	UUID invitedByUserId,
	String invitedByUserName,
	Instant createdAt,
	Instant updatedAt
) {
}
