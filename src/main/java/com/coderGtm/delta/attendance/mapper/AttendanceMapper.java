package com.coderGtm.delta.attendance.mapper;

import org.springframework.stereotype.Component;

import com.coderGtm.delta.attendance.dto.AttendanceEntryResponse;
import com.coderGtm.delta.attendance.entity.AttendanceEntry;
import com.coderGtm.delta.user.User;

/**
 * Maps attendance entities to stable API response payloads.
 */
@Component
public class AttendanceMapper {

	/**
	 * Converts an attendance entity into its external API representation.
	 *
	 * <p>The supplied display name is the outlet-scoped name for the entry's
	 * user; when null it falls back to the user's account name so historical
	 * entries remain readable after membership removal.</p>
	 */
	public AttendanceEntryResponse toResponse(AttendanceEntry entry, String displayName) {
		User user = entry.getUser();

		return new AttendanceEntryResponse(
			entry.getId(),
			entry.getOutlet().getId(),
			user.getId(),
			user.getName(),
			user.getEmail(),
			displayName != null ? displayName : user.getName(),
			entry.getType(),
			entry.getEntryTime(),
			entry.getLatitude(),
			entry.getLongitude(),
			entry.getCreatedBy(),
			entry.getUpdatedBy(),
			entry.getCreatedAt(),
			entry.getUpdatedAt()
		);
	}
}
