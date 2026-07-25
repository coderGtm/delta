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
	 */
	public AttendanceEntryResponse toResponse(AttendanceEntry entry) {
		User user = entry.getUser();

		return new AttendanceEntryResponse(
			entry.getId(),
			entry.getOutlet().getId(),
			user.getId(),
			user.getName(),
			user.getEmail(),
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
