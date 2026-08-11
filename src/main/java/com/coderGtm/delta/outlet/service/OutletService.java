package com.coderGtm.delta.outlet.service;

import java.time.Instant;
import java.util.Map;
import java.util.UUID;

import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.domain.Sort;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import com.coderGtm.delta.common.audit.service.AuditService;
import com.coderGtm.delta.common.dto.PageResponse;
import com.coderGtm.delta.common.exception.BadRequestException;
import com.coderGtm.delta.common.exception.ConflictException;
import com.coderGtm.delta.common.exception.ForbiddenException;
import com.coderGtm.delta.common.exception.ResourceNotFoundException;
import com.coderGtm.delta.common.metrics.ApplicationMetrics;
import com.coderGtm.delta.common.util.PaginationUtils;
import com.coderGtm.delta.outlet.dto.CreateOutletRequest;
import com.coderGtm.delta.outlet.dto.InviteOutletMemberRequest;
import com.coderGtm.delta.outlet.dto.OutletMembershipResponse;
import com.coderGtm.delta.outlet.dto.OutletResponse;
import com.coderGtm.delta.outlet.dto.UpdateOutletGeofenceRequest;
import com.coderGtm.delta.outlet.dto.UpdateMembershipDisplayNameRequest;
import com.coderGtm.delta.outlet.dto.UpdateOutletRequest;
import com.coderGtm.delta.outlet.entity.Outlet;
import com.coderGtm.delta.outlet.entity.OutletMembership;
import com.coderGtm.delta.outlet.entity.OutletMembershipStatus;
import com.coderGtm.delta.outlet.entity.OutletRole;
import com.coderGtm.delta.outlet.mapper.OutletMapper;
import com.coderGtm.delta.outlet.repository.OutletMembershipRepository;
import com.coderGtm.delta.outlet.repository.OutletRepository;
import com.coderGtm.delta.user.User;
import com.coderGtm.delta.user.UserRepository;

import lombok.RequiredArgsConstructor;

/**
 * Encapsulates outlet and outlet membership use cases.
 *
 * <p>This service deliberately keeps authorization and invitation lifecycle
 * rules in one place so controllers stay thin and future attendance rules can
 * rely on the same membership semantics.</p>
 */
@Service
@RequiredArgsConstructor
public class OutletService {

	private final OutletRepository outletRepository;
	private final OutletMembershipRepository outletMembershipRepository;
	private final UserRepository userRepository;
	private final OutletMapper outletMapper;
	private final AuditService auditService;
	private final ApplicationMetrics applicationMetrics;

	/**
	 * Creates a new outlet and immediately assigns the creator as an accepted
	 * owner for that outlet.
	 */
	@Transactional
	public OutletResponse createOutlet(UUID currentUserId, CreateOutletRequest request) {
		User currentUser = getActiveUser(currentUserId);

		Outlet outlet = new Outlet();
		outlet.setName(request.name().trim());
		outlet.setLatitude(request.latitude());
		outlet.setLongitude(request.longitude());
		outlet.setRadiusMeters(request.radiusMeters());
		outlet.setGeofenceEnabled(false);

		Outlet savedOutlet = outletRepository.save(outlet);

		OutletMembership ownerMembership = new OutletMembership();
		ownerMembership.setOutlet(savedOutlet);
		ownerMembership.setUser(currentUser);
		ownerMembership.setDisplayName(currentUser.getName());
		ownerMembership.setRole(OutletRole.OWNER);
		ownerMembership.setStatus(OutletMembershipStatus.ACCEPTED);
		ownerMembership.setInvitedBy(null);
		outletMembershipRepository.save(ownerMembership);
		applicationMetrics.increment("outlet.created");
		auditService.record(
			currentUser.getId(),
			"OUTLET_CREATED",
			"OUTLET",
			savedOutlet.getId(),
			Map.of("name", savedOutlet.getName())
		);

		return outletMapper.toOutletResponse(savedOutlet);
	}

	/**
	 * Returns outlet details for the current user if they have already accepted a
	 * membership in that outlet.
	 */
	@Transactional(readOnly = true)
	public OutletResponse getOutlet(UUID currentUserId, UUID outletId) {
		OutletMembership membership = getMembershipOrThrow(outletId, currentUserId);
		ensureAcceptedMembership(membership);
		return outletMapper.toOutletResponse(membership.getOutlet());
	}

	/**
	 * Updates the core editable details of an outlet.
	 */
	@Transactional
	public OutletResponse updateOutlet(UUID currentUserId, UUID outletId, UpdateOutletRequest request) {
		assertAcceptedOwner(outletId, currentUserId);

		Outlet outlet = outletRepository.findById(outletId)
			.orElseThrow(() -> new ResourceNotFoundException("Outlet not found: " + outletId));

		outlet.setName(request.name().trim());
		outlet.setLatitude(request.latitude());
		outlet.setLongitude(request.longitude());
		outlet.setRadiusMeters(request.radiusMeters());

		Outlet savedOutlet = outletRepository.save(outlet);
		applicationMetrics.increment("outlet.updated");
		auditService.record(currentUserId, "OUTLET_UPDATED", "OUTLET", outletId, Map.of("name", savedOutlet.getName()));
		return outletMapper.toOutletResponse(savedOutlet);
	}

	/**
	 * Toggles attendance geofence enforcement for an outlet. Only accepted outlet
	 * owners may perform this action.
	 */
	@Transactional
	public OutletResponse updateOutletGeofence(UUID currentUserId, UUID outletId, UpdateOutletGeofenceRequest request) {
		assertAcceptedOwner(outletId, currentUserId);

		Outlet outlet = outletRepository.findById(outletId)
			.orElseThrow(() -> new ResourceNotFoundException("Outlet not found: " + outletId));

		outlet.setGeofenceEnabled(Boolean.TRUE.equals(request.geofenceEnabled()));
		Outlet savedOutlet = outletRepository.save(outlet);
		applicationMetrics.increment("outlet.geofence.updated", "enabled", String.valueOf(savedOutlet.isGeofenceEnabled()));
		auditService.record(
			currentUserId,
			"OUTLET_GEOFENCE_UPDATED",
			"OUTLET",
			outletId,
			Map.of("geofenceEnabled", savedOutlet.isGeofenceEnabled())
		);
		return outletMapper.toOutletResponse(savedOutlet);
	}

	/**
	 * Lists all outlets that the current user has accepted membership in.
	 */
	@Transactional(readOnly = true)
	public PageResponse<OutletMembershipResponse> getMyOutlets(UUID currentUserId, Pageable pageable) {
		Page<OutletMembership> memberships = outletMembershipRepository.findAllByUser_IdAndStatusAndRemovedAtIsNull(
			currentUserId,
			OutletMembershipStatus.ACCEPTED,
			PaginationUtils.withDefaultSort(pageable, Sort.by(Sort.Direction.DESC, "updatedAt"))
		);

		return PaginationUtils.toPageResponse(memberships, outletMapper::toMembershipResponse);
	}

	/**
	 * Lists all pending outlet invitations for the current user.
	 */
	@Transactional(readOnly = true)
	public PageResponse<OutletMembershipResponse> getMyInvites(UUID currentUserId, Pageable pageable) {
		Page<OutletMembership> memberships = outletMembershipRepository.findAllByUser_IdAndStatusAndRemovedAtIsNull(
			currentUserId,
			OutletMembershipStatus.INVITED,
			PaginationUtils.withDefaultSort(pageable, Sort.by(Sort.Direction.DESC, "updatedAt"))
		);

		return PaginationUtils.toPageResponse(memberships, outletMapper::toMembershipResponse);
	}

	/**
	 * Lists all memberships for an outlet. Only accepted owners may call this.
	 */
	@Transactional(readOnly = true)
	public PageResponse<OutletMembershipResponse> getOutletMemberships(UUID currentUserId, UUID outletId, Pageable pageable) {
		assertAcceptedOwner(outletId, currentUserId);

		Page<OutletMembership> memberships = outletMembershipRepository.findAllByOutlet_IdAndRemovedAtIsNull(
			outletId,
			PaginationUtils.withDefaultSort(pageable, Sort.by(Sort.Direction.ASC, "createdAt"))
		);

		return PaginationUtils.toPageResponse(memberships, outletMapper::toMembershipResponse);
	}

	/**
	 * Invites an existing user to join the outlet as an employee.
	 *
	 * <p>If the user had previously rejected the outlet, the invitation is
	 * reopened by moving the status back to {@code INVITED}.</p>
	 */
	@Transactional
	public OutletMembershipResponse inviteMember(UUID currentUserId, UUID outletId, InviteOutletMemberRequest request) {
		User inviter = assertAcceptedOwner(outletId, currentUserId);
		User invitee = userRepository.findByEmailIgnoreCaseAndDeletedAtIsNull(normalizeEmail(request.email()))
			.orElseThrow(() -> new ResourceNotFoundException("No active user found for email: " + request.email().trim()));

		OutletMembership existingMembership = outletMembershipRepository.findByOutlet_IdAndUser_Id(outletId, invitee.getId())
			.orElse(null);

		if (existingMembership != null) {
			if (existingMembership.getRemovedAt() == null && existingMembership.getStatus() == OutletMembershipStatus.ACCEPTED) {
				throw new ConflictException("User is already an active member of this outlet");
			}

			if (existingMembership.getRemovedAt() == null && existingMembership.getStatus() == OutletMembershipStatus.INVITED) {
				throw new ConflictException("User already has a pending invitation for this outlet");
			}

			existingMembership.setRole(OutletRole.EMPLOYEE);
			existingMembership.setStatus(OutletMembershipStatus.INVITED);
			existingMembership.setInvitedBy(inviter);
			existingMembership.setRemovedAt(null);
			existingMembership.setRemovedBy(null);

			try {
				OutletMembership savedMembership = outletMembershipRepository.save(existingMembership);
				recordMemberInvited(inviter, savedMembership);
				return outletMapper.toMembershipResponse(getMembershipDetails(savedMembership.getId()));
			} catch (DataIntegrityViolationException ex) {
				throw new ConflictException("User already has a membership record for this outlet");
			}
		}

		Outlet outlet = outletRepository.findById(outletId)
			.orElseThrow(() -> new ResourceNotFoundException("Outlet not found: " + outletId));

		OutletMembership membership = new OutletMembership();
		membership.setOutlet(outlet);
		membership.setUser(invitee);
		membership.setDisplayName(invitee.getName());
		membership.setRole(OutletRole.EMPLOYEE);
		membership.setStatus(OutletMembershipStatus.INVITED);
		membership.setInvitedBy(inviter);
		membership.setRemovedAt(null);
		membership.setRemovedBy(null);

		try {
			OutletMembership savedMembership = outletMembershipRepository.save(membership);
			recordMemberInvited(inviter, savedMembership);
			return outletMapper.toMembershipResponse(getMembershipDetails(savedMembership.getId()));
		} catch (DataIntegrityViolationException ex) {
			throw new ConflictException("User already has a membership record for this outlet");
		}
	}

	/**
	 * Sets the owner-controlled display name for a member of an outlet. Only
	 * accepted outlet owners may perform this action.
	 */
	@Transactional
	public OutletMembershipResponse updateMemberDisplayName(
		UUID currentUserId,
		UUID outletId,
		UUID membershipId,
		UpdateMembershipDisplayNameRequest request
	) {
		assertAcceptedOwner(outletId, currentUserId);
		OutletMembership membership = getMembershipDetails(membershipId);

		if (!membership.getOutlet().getId().equals(outletId)) {
			throw new BadRequestException("The provided membership does not belong to the requested outlet");
		}

		String displayName = request.displayName().trim();
		membership.setDisplayName(displayName);
		OutletMembership savedMembership = outletMembershipRepository.save(membership);
		applicationMetrics.increment("outlet.membership.display_name.updated");
		auditService.record(
			currentUserId,
			"OUTLET_MEMBERSHIP_DISPLAY_NAME_UPDATED",
			"OUTLET_MEMBERSHIP",
			membershipId,
			Map.of("outletId", outletId, "userId", membership.getUser().getId(), "displayName", displayName)
		);
		return outletMapper.toMembershipResponse(savedMembership);
	}

	/**
	 * Accepts an invitation for the current user.
	 */
	@Transactional
	public OutletMembershipResponse acceptInvite(UUID currentUserId, UUID membershipId) {
		OutletMembership membership = getMembershipDetails(membershipId);
		assertInviteTarget(membership, currentUserId);

		if (membership.getStatus() != OutletMembershipStatus.INVITED) {
			throw new BadRequestException("Only pending invitations can be accepted");
		}

		membership.setStatus(OutletMembershipStatus.ACCEPTED);
		OutletMembership savedMembership = outletMembershipRepository.save(membership);
		applicationMetrics.increment("outlet.membership.accepted");
		auditService.record(
			currentUserId,
			"OUTLET_INVITE_ACCEPTED",
			"OUTLET_MEMBERSHIP",
			membershipId,
			Map.of("outletId", savedMembership.getOutlet().getId())
		);
		return outletMapper.toMembershipResponse(savedMembership);
	}

	/**
	 * Rejects an invitation for the current user.
	 */
	@Transactional
	public OutletMembershipResponse rejectInvite(UUID currentUserId, UUID membershipId) {
		OutletMembership membership = getMembershipDetails(membershipId);
		assertInviteTarget(membership, currentUserId);

		if (membership.getStatus() != OutletMembershipStatus.INVITED) {
			throw new BadRequestException("Only pending invitations can be rejected");
		}

		membership.setStatus(OutletMembershipStatus.REJECTED);
		OutletMembership savedMembership = outletMembershipRepository.save(membership);
		applicationMetrics.increment("outlet.membership.rejected");
		auditService.record(
			currentUserId,
			"OUTLET_INVITE_REJECTED",
			"OUTLET_MEMBERSHIP",
			membershipId,
			Map.of("outletId", savedMembership.getOutlet().getId())
		);
		return outletMapper.toMembershipResponse(savedMembership);
	}

	/**
	 * Soft-removes an employee membership so outlet access is revoked while
	 * historical attendance entries can still remain associated with the same
	 * user and outlet.
	 */
	@Transactional
	public void removeMembership(UUID currentUserId, UUID outletId, UUID membershipId) {
		User owner = assertAcceptedOwner(outletId, currentUserId);
		OutletMembership membership = getMembershipDetails(membershipId);

		if (!membership.getOutlet().getId().equals(outletId)) {
			throw new BadRequestException("The provided membership does not belong to the requested outlet");
		}

		if (membership.getRole() == OutletRole.OWNER) {
			throw new BadRequestException("Owner memberships cannot be removed through this endpoint");
		}

		membership.setRemovedAt(Instant.now());
		membership.setRemovedBy(owner);
		outletMembershipRepository.save(membership);
		applicationMetrics.increment("outlet.membership.removed");
		auditService.record(
			currentUserId,
			"OUTLET_MEMBERSHIP_REMOVED",
			"OUTLET_MEMBERSHIP",
			membershipId,
			Map.of("outletId", outletId, "removedUserId", membership.getUser().getId())
		);
	}

	private void recordMemberInvited(User inviter, OutletMembership membership) {
		applicationMetrics.increment("outlet.membership.invited");
		auditService.record(
			inviter.getId(),
			"OUTLET_MEMBER_INVITED",
			"OUTLET_MEMBERSHIP",
			membership.getId(),
			Map.of("outletId", membership.getOutlet().getId(), "inviteeUserId", membership.getUser().getId())
		);
	}

	private User getActiveUser(UUID userId) {
		return userRepository.findByIdAndDeletedAtIsNull(userId)
			.orElseThrow(() -> new ResourceNotFoundException("Authenticated user was not found"));
	}

	private User assertAcceptedOwner(UUID outletId, UUID currentUserId) {
		OutletMembership membership = getMembershipOrThrow(outletId, currentUserId);
		ensureAcceptedMembership(membership);

		if (membership.getRole() != OutletRole.OWNER) {
			throw new ForbiddenException("Only outlet owners can perform this action");
		}

		return membership.getUser();
	}

	private OutletMembership getMembershipOrThrow(UUID outletId, UUID userId) {
		return outletMembershipRepository.findByOutlet_IdAndUser_IdAndRemovedAtIsNull(outletId, userId)
			.orElseThrow(() -> new ResourceNotFoundException("Outlet membership was not found for the current user"));
	}

	private OutletMembership getMembershipDetails(UUID membershipId) {
		return outletMembershipRepository.findDetailedByIdAndRemovedAtIsNull(membershipId)
			.orElseThrow(() -> new ResourceNotFoundException("Outlet membership not found: " + membershipId));
	}

	private void ensureAcceptedMembership(OutletMembership membership) {
		if (membership.getStatus() != OutletMembershipStatus.ACCEPTED) {
			throw new ForbiddenException("You must accept the outlet invitation before accessing this outlet");
		}
	}

	private void assertInviteTarget(OutletMembership membership, UUID currentUserId) {
		if (!membership.getUser().getId().equals(currentUserId)) {
			throw new ForbiddenException("You can only manage your own outlet invitations");
		}
	}

	private String normalizeEmail(String email) {
		return email == null ? null : email.trim().toLowerCase();
	}
}
