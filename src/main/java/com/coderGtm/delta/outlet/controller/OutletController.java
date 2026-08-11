package com.coderGtm.delta.outlet.controller;

import java.util.UUID;

import org.springframework.data.domain.Pageable;
import org.springframework.data.web.PageableDefault;
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
import org.springframework.web.bind.annotation.RestController;

import com.coderGtm.delta.common.dto.PageResponse;
import com.coderGtm.delta.common.web.ApiPaths;
import com.coderGtm.delta.outlet.dto.CreateOutletRequest;
import com.coderGtm.delta.outlet.dto.InviteOutletMemberRequest;
import com.coderGtm.delta.outlet.dto.OutletMembershipResponse;
import com.coderGtm.delta.outlet.dto.OutletResponse;
import com.coderGtm.delta.outlet.dto.UpdateOutletGeofenceRequest;
import com.coderGtm.delta.outlet.dto.UpdateMembershipDisplayNameRequest;
import com.coderGtm.delta.outlet.dto.UpdateOutletRequest;
import com.coderGtm.delta.outlet.service.OutletService;
import com.coderGtm.delta.user.User;

import jakarta.validation.Valid;
import lombok.RequiredArgsConstructor;

/**
 * REST endpoints for outlet management and membership invitations.
 */
@RestController
@RequestMapping(ApiPaths.OUTLETS)
@RequiredArgsConstructor
@Validated
public class OutletController {

	private final OutletService outletService;

	/**
	 * Creates a new outlet owned by the authenticated user.
	 */
	@PostMapping
	public ResponseEntity<OutletResponse> createOutlet(
		@AuthenticationPrincipal User currentUser,
		@Valid @RequestBody CreateOutletRequest request
	) {
		return ResponseEntity.status(HttpStatus.CREATED)
			.body(outletService.createOutlet(currentUser.getId(), request));
	}

	/**
	 * Returns a single outlet if the authenticated user is an accepted member.
	 */
	@GetMapping("/{outletId}")
	public ResponseEntity<OutletResponse> getOutlet(
		@AuthenticationPrincipal User currentUser,
		@PathVariable UUID outletId
	) {
		return ResponseEntity.ok(outletService.getOutlet(currentUser.getId(), outletId));
	}

	/**
	 * Updates outlet details. Only accepted outlet owners may perform this action.
	 */
	@PutMapping("/{outletId}")
	public ResponseEntity<OutletResponse> updateOutlet(
		@AuthenticationPrincipal User currentUser,
		@PathVariable UUID outletId,
		@Valid @RequestBody UpdateOutletRequest request
	) {
		return ResponseEntity.ok(outletService.updateOutlet(currentUser.getId(), outletId, request));
	}

	/**
	 * Toggles attendance geofence enforcement for an outlet. Only accepted outlet
	 * owners may perform this action.
	 */
	@PutMapping("/{outletId}/geofence")
	public ResponseEntity<OutletResponse> updateOutletGeofence(
		@AuthenticationPrincipal User currentUser,
		@PathVariable UUID outletId,
		@Valid @RequestBody UpdateOutletGeofenceRequest request
	) {
		return ResponseEntity.ok(outletService.updateOutletGeofence(currentUser.getId(), outletId, request));
	}

	/**
	 * Lists all outlets that the authenticated user has already joined.
	 */
	@GetMapping("/mine")
	public ResponseEntity<PageResponse<OutletMembershipResponse>> getMyOutlets(
		@AuthenticationPrincipal User currentUser,
		@PageableDefault(size = 20) Pageable pageable
	) {
		return ResponseEntity.ok(outletService.getMyOutlets(currentUser.getId(), pageable));
	}

	/**
	 * Lists all pending outlet invitations for the authenticated user.
	 */
	@GetMapping("/invites")
	public ResponseEntity<PageResponse<OutletMembershipResponse>> getMyInvites(
		@AuthenticationPrincipal User currentUser,
		@PageableDefault(size = 20) Pageable pageable
	) {
		return ResponseEntity.ok(outletService.getMyInvites(currentUser.getId(), pageable));
	}

	/**
	 * Lists all memberships for an outlet. Only accepted owners can access it.
	 */
	@GetMapping("/{outletId}/memberships")
	public ResponseEntity<PageResponse<OutletMembershipResponse>> getOutletMemberships(
		@AuthenticationPrincipal User currentUser,
		@PathVariable UUID outletId,
		@PageableDefault(size = 20) Pageable pageable
	) {
		return ResponseEntity.ok(outletService.getOutletMemberships(currentUser.getId(), outletId, pageable));
	}

	/**
	 * Sends an invitation to an existing user to join the outlet as an employee.
	 */
	@PostMapping("/{outletId}/memberships/invite")
	public ResponseEntity<OutletMembershipResponse> inviteMember(
		@AuthenticationPrincipal User currentUser,
		@PathVariable UUID outletId,
		@Valid @RequestBody InviteOutletMemberRequest request
	) {
		return ResponseEntity.status(HttpStatus.CREATED)
			.body(outletService.inviteMember(currentUser.getId(), outletId, request));
	}

	/**
	 * Removes an employee membership from an outlet without deleting historical
	 * records that may point to the user and outlet.
	 */
	@DeleteMapping("/{outletId}/memberships/{membershipId}")
	public ResponseEntity<Void> removeMembership(
		@AuthenticationPrincipal User currentUser,
		@PathVariable UUID outletId,
		@PathVariable UUID membershipId
	) {
		outletService.removeMembership(currentUser.getId(), outletId, membershipId);
		return ResponseEntity.noContent().build();
	}

	/**
	 * Sets the owner-controlled display name for a member of an outlet. Only
	 * accepted outlet owners may perform this action.
	 */
	@PutMapping("/{outletId}/memberships/{membershipId}/display-name")
	public ResponseEntity<OutletMembershipResponse> updateMemberDisplayName(
		@AuthenticationPrincipal User currentUser,
		@PathVariable UUID outletId,
		@PathVariable UUID membershipId,
		@Valid @RequestBody UpdateMembershipDisplayNameRequest request
	) {
		return ResponseEntity.ok(
			outletService.updateMemberDisplayName(currentUser.getId(), outletId, membershipId, request)
		);
	}

	/**
	 * Accepts a pending outlet invitation for the authenticated user.
	 */
	@PostMapping("/memberships/{membershipId}/accept")
	public ResponseEntity<OutletMembershipResponse> acceptInvite(
		@AuthenticationPrincipal User currentUser,
		@PathVariable UUID membershipId
	) {
		return ResponseEntity.ok(outletService.acceptInvite(currentUser.getId(), membershipId));
	}

	/**
	 * Rejects a pending outlet invitation for the authenticated user.
	 */
	@PostMapping("/memberships/{membershipId}/reject")
	public ResponseEntity<OutletMembershipResponse> rejectInvite(
		@AuthenticationPrincipal User currentUser,
		@PathVariable UUID membershipId
	) {
		return ResponseEntity.ok(outletService.rejectInvite(currentUser.getId(), membershipId));
	}
}
