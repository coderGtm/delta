package com.coderGtm.delta.attendance.dto;

import java.math.BigDecimal;
import java.time.Instant;
import java.util.UUID;

import com.coderGtm.delta.attendance.entity.AttendanceEntryType;

/**
 * API representation of an attendance entry, including user context and audit
 * metadata.
 *
 * <p>{@code displayName} is the outlet-scoped owner-controlled name to render
 * for the entry's user; it falls back to the user's account name when the user
 * is no longer an active member.</p>
 */
public record AttendanceEntryResponse(
	UUID id,
	UUID outletId,
	UUID userId,
	String userName,
	String userEmail,
	String displayName,
	AttendanceEntryType type,
	Instant entryTime,
	BigDecimal latitude,
	BigDecimal longitude,
	UUID createdByUserId,
	UUID updatedByUserId,
	Instant createdAt,
	Instant updatedAt
) {
}
