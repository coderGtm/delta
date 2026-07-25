package com.coderGtm.delta.attendance.controller;

import java.util.List;
import java.util.UUID;

import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.security.core.annotation.AuthenticationPrincipal;
import org.springframework.validation.annotation.Validated;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

import com.coderGtm.delta.attendance.dto.AttendanceEntryResponse;
import com.coderGtm.delta.attendance.dto.CreateAttendanceEntryRequest;
import com.coderGtm.delta.attendance.dto.ManageAttendanceEntryRequest;
import com.coderGtm.delta.attendance.dto.UpdateAttendanceEntryRequest;
import com.coderGtm.delta.attendance.service.AttendanceService;
import com.coderGtm.delta.user.User;

import jakarta.validation.Valid;
import lombok.RequiredArgsConstructor;

/**
 * REST endpoints for outlet attendance management.
 */
@RestController
@RequestMapping("/outlets")
@RequiredArgsConstructor
@Validated
public class AttendanceController {

	private final AttendanceService attendanceService;

	/**
	 * Creates an attendance entry for the authenticated employee using the
	 * current server time.
	 */
	@PostMapping("/{outletId}/attendance")
	public ResponseEntity<AttendanceEntryResponse> createOwnEntry(
		@AuthenticationPrincipal User currentUser,
		@PathVariable UUID outletId,
		@Valid @RequestBody CreateAttendanceEntryRequest request
	) {
		return ResponseEntity.status(HttpStatus.CREATED)
			.body(attendanceService.createOwnEntry(currentUser.getId(), outletId, request));
	}

	/**
	 * Creates an attendance entry for an employee on behalf of an outlet owner.
	 */
	@PostMapping("/{outletId}/attendance/manage")
	public ResponseEntity<AttendanceEntryResponse> createManagedEntry(
		@AuthenticationPrincipal User currentUser,
		@PathVariable UUID outletId,
		@Valid @RequestBody ManageAttendanceEntryRequest request
	) {
		return ResponseEntity.status(HttpStatus.CREATED)
			.body(attendanceService.createManagedEntry(currentUser.getId(), outletId, request));
	}

	/**
	 * Lists attendance entries for the outlet. Owners may optionally filter by
	 * user, while employees only receive their own records.
	 */
	@GetMapping("/{outletId}/attendance")
	public ResponseEntity<List<AttendanceEntryResponse>> getAttendanceEntries(
		@AuthenticationPrincipal User currentUser,
		@PathVariable UUID outletId,
		@RequestParam(required = false) UUID userId
	) {
		return ResponseEntity.ok(attendanceService.getAttendanceEntries(currentUser.getId(), outletId, userId));
	}

	/**
	 * Returns a single attendance entry if the current caller is authorized to
	 * view it.
	 */
	@GetMapping("/{outletId}/attendance/{attendanceEntryId}")
	public ResponseEntity<AttendanceEntryResponse> getAttendanceEntry(
		@AuthenticationPrincipal User currentUser,
		@PathVariable UUID outletId,
		@PathVariable UUID attendanceEntryId
	) {
		return ResponseEntity.ok(
			attendanceService.getAttendanceEntry(currentUser.getId(), outletId, attendanceEntryId)
		);
	}

	/**
	 * Updates an attendance entry. Only outlet owners may perform this action.
	 */
	@PutMapping("/{outletId}/attendance/{attendanceEntryId}")
	public ResponseEntity<AttendanceEntryResponse> updateAttendanceEntry(
		@AuthenticationPrincipal User currentUser,
		@PathVariable UUID outletId,
		@PathVariable UUID attendanceEntryId,
		@Valid @RequestBody UpdateAttendanceEntryRequest request
	) {
		return ResponseEntity.ok(
			attendanceService.updateAttendanceEntry(currentUser.getId(), outletId, attendanceEntryId, request)
		);
	}

	/**
	 * Deletes an attendance entry. Only outlet owners may perform this action.
	 */
	@DeleteMapping("/{outletId}/attendance/{attendanceEntryId}")
	public ResponseEntity<Void> deleteAttendanceEntry(
		@AuthenticationPrincipal User currentUser,
		@PathVariable UUID outletId,
		@PathVariable UUID attendanceEntryId
	) {
		attendanceService.deleteAttendanceEntry(currentUser.getId(), outletId, attendanceEntryId);
		return ResponseEntity.noContent().build();
	}
}
