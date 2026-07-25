package com.coderGtm.delta.attendance.dto;

import java.math.BigDecimal;
import java.time.Instant;
import java.util.UUID;

import com.coderGtm.delta.attendance.entity.AttendanceEntryType;

/**
 * API representation of an attendance entry, including user context and audit
 * metadata.
 */
public record AttendanceEntryResponse(
	UUID id,
	UUID outletId,
	UUID userId,
	String userName,
	String userEmail,
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
