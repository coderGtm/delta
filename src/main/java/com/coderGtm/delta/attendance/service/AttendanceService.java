package com.coderGtm.delta.attendance.service;

import java.time.Clock;
import java.time.Instant;
import java.util.Map;
import java.util.UUID;
import java.util.stream.Collectors;

import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.domain.Sort;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import com.coderGtm.delta.attendance.dto.AttendanceEntryResponse;
import com.coderGtm.delta.attendance.dto.CreateAttendanceEntryRequest;
import com.coderGtm.delta.attendance.dto.ManageAttendanceEntryRequest;
import com.coderGtm.delta.attendance.dto.UpdateAttendanceEntryRequest;
import com.coderGtm.delta.attendance.entity.AttendanceEntry;
import com.coderGtm.delta.attendance.mapper.AttendanceMapper;
import com.coderGtm.delta.attendance.repository.AttendanceEntryRepository;
import com.coderGtm.delta.common.audit.service.AuditService;
import com.coderGtm.delta.common.dto.PageResponse;
import com.coderGtm.delta.common.exception.BadRequestException;
import com.coderGtm.delta.common.exception.ForbiddenException;
import com.coderGtm.delta.common.exception.ResourceNotFoundException;
import com.coderGtm.delta.common.metrics.ApplicationMetrics;
import com.coderGtm.delta.common.util.GeoUtils;
import com.coderGtm.delta.common.util.PaginationUtils;
import com.coderGtm.delta.outlet.entity.OutletMembership;
import com.coderGtm.delta.outlet.entity.OutletMembershipStatus;
import com.coderGtm.delta.outlet.entity.OutletRole;
import com.coderGtm.delta.outlet.repository.OutletMembershipRepository;

import lombok.RequiredArgsConstructor;

/**
 * Encapsulates attendance creation and access rules for outlet owners and
 * employees.
 *
 * <p>Authorization is always based on the caller's current active outlet
 * membership, while historical attendance records remain readable and editable
 * without re-checking the subject employee's current membership.</p>
 */
@Service
@RequiredArgsConstructor
public class AttendanceService {

	private final AttendanceEntryRepository attendanceEntryRepository;
	private final OutletMembershipRepository outletMembershipRepository;
	private final AttendanceMapper attendanceMapper;
	private final Clock clock;
	private final AuditService auditService;
	private final ApplicationMetrics applicationMetrics;

	/**
	 * Creates a new attendance entry for the authenticated employee using the
	 * current server-side UTC timestamp.
	 */
	@Transactional
	public AttendanceEntryResponse createOwnEntry(UUID currentUserId, UUID outletId, CreateAttendanceEntryRequest request) {
		OutletMembership currentMembership = assertAcceptedCurrentMembership(outletId, currentUserId);
		assertActiveOutlet(currentMembership.getOutlet());

		if (currentMembership.getRole() != OutletRole.EMPLOYEE) {
			throw new ForbiddenException("Only accepted employees can create their own attendance entries");
		}

		validateGeofenceIfEnabled(currentMembership.getOutlet(), request.latitude(), request.longitude());

		AttendanceEntry entry = new AttendanceEntry();
		entry.setUser(currentMembership.getUser());
		entry.setOutlet(currentMembership.getOutlet());
		entry.setType(request.type());
		entry.setEntryTime(Instant.now(clock));
		entry.setLatitude(request.latitude());
		entry.setLongitude(request.longitude());

		AttendanceEntry savedEntry = attendanceEntryRepository.save(entry);
		applicationMetrics.increment("attendance.created", "mode", "self");
		auditService.record(
			currentUserId,
			"ATTENDANCE_CREATED",
			"ATTENDANCE_ENTRY",
			savedEntry.getId(),
			Map.of("outletId", outletId, "userId", currentUserId, "type", savedEntry.getType().name(), "mode", "self")
		);
		return attendanceMapper.toResponse(savedEntry, currentMembership.getDisplayName());
	}

	/**
	 * Creates a new attendance entry for an employee on behalf of an outlet owner.
	 */
	@Transactional
	public AttendanceEntryResponse createManagedEntry(UUID currentUserId, UUID outletId, ManageAttendanceEntryRequest request) {
		OutletMembership currentMembership = assertAcceptedCurrentMembership(outletId, currentUserId);
		assertOwner(currentMembership);
		assertActiveOutlet(currentMembership.getOutlet());

		OutletMembership targetMembership = getActiveMembership(
			outletId,
			request.userId(),
			"Outlet membership was not found for the requested user"
		);

		if (targetMembership.getStatus() != OutletMembershipStatus.ACCEPTED
			|| targetMembership.getRole() != OutletRole.EMPLOYEE) {
			throw new BadRequestException("Attendance can only be created for accepted employee memberships");
		}

		validateGeofenceIfEnabled(targetMembership.getOutlet(), request.latitude(), request.longitude());

		AttendanceEntry entry = new AttendanceEntry();
		entry.setUser(targetMembership.getUser());
		entry.setOutlet(targetMembership.getOutlet());
		entry.setType(request.type());
		entry.setEntryTime(request.entryTime());
		entry.setLatitude(request.latitude());
		entry.setLongitude(request.longitude());

		AttendanceEntry savedEntry = attendanceEntryRepository.save(entry);
		applicationMetrics.increment("attendance.created", "mode", "managed");
		auditService.record(
			currentUserId,
			"ATTENDANCE_CREATED",
			"ATTENDANCE_ENTRY",
			savedEntry.getId(),
			Map.of("outletId", outletId, "userId", request.userId(), "type", savedEntry.getType().name(), "mode", "managed")
		);
		return attendanceMapper.toResponse(savedEntry, targetMembership.getDisplayName());
	}

	/**
	 * Lists attendance entries in an outlet. Owners may view all entries or filter
	 * by user, while employees may only view their own entries.
	 */
	@Transactional(readOnly = true)
	public PageResponse<AttendanceEntryResponse> getAttendanceEntries(
		UUID currentUserId,
		UUID outletId,
		UUID userId,
		Pageable pageable
	) {
		OutletMembership currentMembership = assertAcceptedCurrentMembership(outletId, currentUserId);
		Pageable sortedPageable = PaginationUtils.withDefaultSort(
			pageable,
			Sort.by(Sort.Order.desc("entryTime"), Sort.Order.desc("createdAt"))
		);

		if (currentMembership.getRole() == OutletRole.OWNER) {
			Page<AttendanceEntry> entries = userId == null
				? attendanceEntryRepository.findAllByOutlet_Id(outletId, sortedPageable)
				: attendanceEntryRepository.findAllByOutlet_IdAndUser_Id(outletId, userId, sortedPageable);

			return toPageResponseWithDisplayNames(outletId, entries);
		}

		if (userId != null && !userId.equals(currentUserId)) {
			throw new ForbiddenException("Employees can only view their own attendance entries");
		}

		Page<AttendanceEntry> entries = attendanceEntryRepository.findAllByOutlet_IdAndUser_Id(
			outletId,
			currentUserId,
			sortedPageable
		);

		return toPageResponseWithDisplayNames(outletId, entries);
	}

	/**
	 * Returns a single attendance entry if the current caller is allowed to view
	 * it.
	 */
	@Transactional(readOnly = true)
	public AttendanceEntryResponse getAttendanceEntry(UUID currentUserId, UUID outletId, UUID attendanceEntryId) {
		OutletMembership currentMembership = assertAcceptedCurrentMembership(outletId, currentUserId);
		AttendanceEntry entry = getAttendanceEntryOrThrow(outletId, attendanceEntryId);

		if (currentMembership.getRole() == OutletRole.OWNER || entry.getUser().getId().equals(currentUserId)) {
			return attendanceMapper.toResponse(entry, resolveMemberDisplayName(outletId, entry));
		}

		throw new ForbiddenException("Employees can only view their own attendance entries");
	}

	/**
	 * Updates an attendance entry. Only accepted outlet owners may perform this
	 * action.
	 */
	@Transactional
	public AttendanceEntryResponse updateAttendanceEntry(
		UUID currentUserId,
		UUID outletId,
		UUID attendanceEntryId,
		UpdateAttendanceEntryRequest request
	) {
		OutletMembership currentMembership = assertAcceptedCurrentMembership(outletId, currentUserId);
		assertOwner(currentMembership);
		assertActiveOutlet(currentMembership.getOutlet());

		AttendanceEntry entry = getAttendanceEntryOrThrow(outletId, attendanceEntryId);
		validateGeofenceIfEnabled(entry.getOutlet(), request.latitude(), request.longitude());
		entry.setType(request.type());
		entry.setEntryTime(request.entryTime());
		entry.setLatitude(request.latitude());
		entry.setLongitude(request.longitude());

		AttendanceEntry savedEntry = attendanceEntryRepository.save(entry);
		applicationMetrics.increment("attendance.updated");
		auditService.record(
			currentUserId,
			"ATTENDANCE_UPDATED",
			"ATTENDANCE_ENTRY",
			attendanceEntryId,
			Map.of("outletId", outletId, "userId", savedEntry.getUser().getId(), "type", savedEntry.getType().name())
		);
		return attendanceMapper.toResponse(savedEntry, resolveMemberDisplayName(outletId, savedEntry));
	}

	/**
	 * Deletes an attendance entry. Only accepted outlet owners may perform this
	 * action.
	 */
	@Transactional
	public void deleteAttendanceEntry(UUID currentUserId, UUID outletId, UUID attendanceEntryId) {
		OutletMembership currentMembership = assertAcceptedCurrentMembership(outletId, currentUserId);
		assertOwner(currentMembership);
		assertActiveOutlet(currentMembership.getOutlet());
		AttendanceEntry entry = getAttendanceEntryOrThrow(outletId, attendanceEntryId);
		attendanceEntryRepository.delete(entry);
		applicationMetrics.increment("attendance.deleted");
		auditService.record(
			currentUserId,
			"ATTENDANCE_DELETED",
			"ATTENDANCE_ENTRY",
			attendanceEntryId,
			Map.of("outletId", outletId, "userId", entry.getUser().getId(), "type", entry.getType().name())
		);
	}

	private OutletMembership assertAcceptedCurrentMembership(UUID outletId, UUID currentUserId) {
		OutletMembership membership = getActiveMembership(
			outletId,
			currentUserId,
			"Outlet membership was not found for the current user"
		);

		if (membership.getStatus() != OutletMembershipStatus.ACCEPTED) {
			throw new ForbiddenException("You must accept the outlet invitation before accessing this outlet");
		}

		return membership;
	}

	private OutletMembership getActiveMembership(UUID outletId, UUID userId, String notFoundMessage) {
		return outletMembershipRepository.findByOutlet_IdAndUser_IdAndRemovedAtIsNull(outletId, userId)
			.orElseThrow(() -> new ResourceNotFoundException(notFoundMessage));
	}

	private PageResponse<AttendanceEntryResponse> toPageResponseWithDisplayNames(
		UUID outletId,
		Page<AttendanceEntry> entries
	) {
		Map<UUID, String> displayNames = memberDisplayNames(outletId);
		return PaginationUtils.toPageResponse(
			entries,
			entry -> attendanceMapper.toResponse(entry, displayNames.get(entry.getUser().getId()))
		);
	}

	private Map<UUID, String> memberDisplayNames(UUID outletId) {
		return outletMembershipRepository.findAllByOutlet_Id(outletId).stream()
			.collect(Collectors.toMap(
				membership -> membership.getUser().getId(),
				OutletMembership::getDisplayName,
				(first, second) -> first
			));
	}

	private String resolveMemberDisplayName(UUID outletId, AttendanceEntry entry) {
		return outletMembershipRepository.findByOutlet_IdAndUser_Id(outletId, entry.getUser().getId())
			.map(OutletMembership::getDisplayName)
			.orElse(null);
	}

	private void assertOwner(OutletMembership membership) {
		if (membership.getRole() != OutletRole.OWNER) {
			throw new ForbiddenException("Only outlet owners can perform this action");
		}
	}

	private void assertActiveOutlet(com.coderGtm.delta.outlet.entity.Outlet outlet) {
		if (outlet.getRemovedAt() != null) {
			throw new ResourceNotFoundException("Outlet not found: " + outlet.getId());
		}
	}

	private AttendanceEntry getAttendanceEntryOrThrow(UUID outletId, UUID attendanceEntryId) {
		return attendanceEntryRepository.findDetailedByIdAndOutlet_Id(attendanceEntryId, outletId)
			.orElseThrow(() -> new ResourceNotFoundException("Attendance entry not found: " + attendanceEntryId));
	}

	private void validateGeofenceIfEnabled(
		com.coderGtm.delta.outlet.entity.Outlet outlet,
		java.math.BigDecimal latitude,
		java.math.BigDecimal longitude
	) {
		if (!outlet.isGeofenceEnabled()) {
			return;
		}

		boolean withinRadius = GeoUtils.isWithinRadiusMeters(
			outlet.getLatitude(),
			outlet.getLongitude(),
			latitude,
			longitude,
			outlet.getRadiusMeters()
		);

		if (!withinRadius) {
			applicationMetrics.increment("attendance.geofence.rejected", "outletId", outlet.getId().toString());
			throw new ForbiddenException("Attendance location is outside the outlet geofence");
		}
	}
}
