package com.coderGtm.delta.outlet.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import java.math.BigDecimal;

import java.util.List;
import java.util.Optional;
import java.util.UUID;

import org.junit.jupiter.api.BeforeEach;
import org.springframework.data.domain.PageImpl;
import org.springframework.data.domain.PageRequest;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import com.coderGtm.delta.common.dto.PageResponse;
import com.coderGtm.delta.common.exception.BadRequestException;
import com.coderGtm.delta.common.exception.ConflictException;
import com.coderGtm.delta.common.exception.ForbiddenException;
import com.coderGtm.delta.outlet.dto.CreateOutletRequest;
import com.coderGtm.delta.outlet.dto.InviteOutletMemberRequest;
import com.coderGtm.delta.outlet.dto.OutletMembershipResponse;
import com.coderGtm.delta.outlet.dto.OutletResponse;
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

@ExtendWith(MockitoExtension.class)
class OutletServiceTest {

	@Mock
	private OutletRepository outletRepository;

	@Mock
	private OutletMembershipRepository outletMembershipRepository;

	@Mock
	private UserRepository userRepository;

	private OutletService outletService;

	@BeforeEach
	void setUp() {
		outletService = new OutletService(
			outletRepository,
			outletMembershipRepository,
			userRepository,
			new OutletMapper()
		);
	}

	@Test
	void createOutletCreatesAcceptedOwnerMembership() {
		UUID ownerId = UUID.randomUUID();
		User owner = user(ownerId, "owner@example.com", "Owner");
		Outlet savedOutlet = outlet(UUID.randomUUID(), "HQ");
		when(userRepository.findByIdAndDeletedAtIsNull(ownerId)).thenReturn(Optional.of(owner));
		when(outletRepository.save(any(Outlet.class))).thenReturn(savedOutlet);

		OutletResponse response = outletService.createOutlet(
			ownerId,
			new CreateOutletRequest("HQ", new BigDecimal("12.9715987"), new BigDecimal("77.5945627"), 150)
		);

		assertThat(response.id()).isEqualTo(savedOutlet.getId());
		assertThat(response.name()).isEqualTo("HQ");
		verify(outletMembershipRepository).save(any(OutletMembership.class));
	}

	@Test
	void updateOutletAllowsAcceptedOwnerToChangeDetails() {
		UUID outletId = UUID.randomUUID();
		UUID ownerId = UUID.randomUUID();
		User owner = user(ownerId, "owner@example.com", "Owner");
		Outlet outlet = outlet(outletId, "Old Name");
		OutletMembership ownerMembership = membership(UUID.randomUUID(), outlet, owner, OutletRole.OWNER, OutletMembershipStatus.ACCEPTED);

		when(outletMembershipRepository.findByOutlet_IdAndUser_IdAndRemovedAtIsNull(outletId, ownerId))
			.thenReturn(Optional.of(ownerMembership));
		when(outletRepository.findById(outletId)).thenReturn(Optional.of(outlet));
		when(outletRepository.save(outlet)).thenReturn(outlet);

		OutletResponse response = outletService.updateOutlet(
			ownerId,
			outletId,
			new UpdateOutletRequest("Updated Outlet", new BigDecimal("28.6139000"), new BigDecimal("77.2090000"), 250)
		);

		assertThat(response.name()).isEqualTo("Updated Outlet");
		assertThat(response.latitude()).isEqualByComparingTo("28.6139000");
		assertThat(response.radiusMeters()).isEqualTo(250);
	}

	@Test
	void inviteMemberCreatesPendingInvitationForExistingUser() {
		UUID outletId = UUID.randomUUID();
		UUID ownerId = UUID.randomUUID();
		UUID employeeId = UUID.randomUUID();
		User owner = user(ownerId, "owner@example.com", "Owner");
		User employee = user(employeeId, "employee@example.com", "Employee");
		Outlet outlet = outlet(outletId, "Outlet A");
		OutletMembership ownerMembership = membership(UUID.randomUUID(), outlet, owner, OutletRole.OWNER, OutletMembershipStatus.ACCEPTED);
		OutletMembership savedInvite = membership(UUID.randomUUID(), outlet, employee, OutletRole.EMPLOYEE, OutletMembershipStatus.INVITED);
		savedInvite.setInvitedBy(owner);

		when(outletMembershipRepository.findByOutlet_IdAndUser_IdAndRemovedAtIsNull(outletId, ownerId))
			.thenReturn(Optional.of(ownerMembership));
		when(userRepository.findByEmailIgnoreCaseAndDeletedAtIsNull("employee@example.com")).thenReturn(Optional.of(employee));
		when(outletMembershipRepository.findByOutlet_IdAndUser_Id(outletId, employeeId)).thenReturn(Optional.empty());
		when(outletRepository.findById(outletId)).thenReturn(Optional.of(outlet));
		when(outletMembershipRepository.save(any(OutletMembership.class))).thenReturn(savedInvite);
		when(outletMembershipRepository.findDetailedByIdAndRemovedAtIsNull(savedInvite.getId())).thenReturn(Optional.of(savedInvite));

		OutletMembershipResponse response = outletService.inviteMember(
			ownerId,
			outletId,
			new InviteOutletMemberRequest(" Employee@Example.com ")
		);

		assertThat(response.status()).isEqualTo(OutletMembershipStatus.INVITED);
		assertThat(response.userEmail()).isEqualTo("employee@example.com");
		assertThat(response.invitedByUserId()).isEqualTo(ownerId);
	}

	@Test
	void inviteMemberRejectsNonOwners() {
		UUID outletId = UUID.randomUUID();
		UUID employeeId = UUID.randomUUID();
		Outlet outlet = outlet(outletId, "Outlet A");
		User employee = user(employeeId, "employee@example.com", "Employee");
		OutletMembership employeeMembership = membership(
			UUID.randomUUID(),
			outlet,
			employee,
			OutletRole.EMPLOYEE,
			OutletMembershipStatus.ACCEPTED
		);
		when(outletMembershipRepository.findByOutlet_IdAndUser_IdAndRemovedAtIsNull(outletId, employeeId))
			.thenReturn(Optional.of(employeeMembership));

		assertThatThrownBy(() -> outletService.inviteMember(
			employeeId,
			outletId,
			new InviteOutletMemberRequest("someone@example.com")
		))
			.isInstanceOf(ForbiddenException.class)
			.hasMessage("Only outlet owners can perform this action");
	}

	@Test
	void inviteMemberConflictsWhenUserIsAlreadyAccepted() {
		UUID outletId = UUID.randomUUID();
		UUID ownerId = UUID.randomUUID();
		UUID employeeId = UUID.randomUUID();
		User owner = user(ownerId, "owner@example.com", "Owner");
		User employee = user(employeeId, "employee@example.com", "Employee");
		Outlet outlet = outlet(outletId, "Outlet A");
		OutletMembership ownerMembership = membership(UUID.randomUUID(), outlet, owner, OutletRole.OWNER, OutletMembershipStatus.ACCEPTED);
		OutletMembership acceptedEmployee = membership(UUID.randomUUID(), outlet, employee, OutletRole.EMPLOYEE, OutletMembershipStatus.ACCEPTED);
		when(outletMembershipRepository.findByOutlet_IdAndUser_IdAndRemovedAtIsNull(outletId, ownerId))
			.thenReturn(Optional.of(ownerMembership));
		when(userRepository.findByEmailIgnoreCaseAndDeletedAtIsNull("employee@example.com")).thenReturn(Optional.of(employee));
		when(outletMembershipRepository.findByOutlet_IdAndUser_Id(outletId, employeeId)).thenReturn(Optional.of(acceptedEmployee));

		assertThatThrownBy(() -> outletService.inviteMember(
			ownerId,
			outletId,
			new InviteOutletMemberRequest("employee@example.com")
		))
			.isInstanceOf(ConflictException.class)
			.hasMessage("User is already an active member of this outlet");
	}

	@Test
	void acceptInviteTransitionsMembershipToAccepted() {
		UUID membershipId = UUID.randomUUID();
		UUID employeeId = UUID.randomUUID();
		User employee = user(employeeId, "employee@example.com", "Employee");
		Outlet outlet = outlet(UUID.randomUUID(), "Outlet A");
		OutletMembership invite = membership(membershipId, outlet, employee, OutletRole.EMPLOYEE, OutletMembershipStatus.INVITED);
		when(outletMembershipRepository.findDetailedByIdAndRemovedAtIsNull(membershipId)).thenReturn(Optional.of(invite));
		when(outletMembershipRepository.save(invite)).thenReturn(invite);

		OutletMembershipResponse response = outletService.acceptInvite(employeeId, membershipId);

		assertThat(response.status()).isEqualTo(OutletMembershipStatus.ACCEPTED);
	}

	@Test
	void rejectInviteFailsWhenMembershipIsNotPending() {
		UUID membershipId = UUID.randomUUID();
		UUID employeeId = UUID.randomUUID();
		User employee = user(employeeId, "employee@example.com", "Employee");
		Outlet outlet = outlet(UUID.randomUUID(), "Outlet A");
		OutletMembership membership = membership(membershipId, outlet, employee, OutletRole.EMPLOYEE, OutletMembershipStatus.REJECTED);
		when(outletMembershipRepository.findDetailedByIdAndRemovedAtIsNull(membershipId)).thenReturn(Optional.of(membership));

		assertThatThrownBy(() -> outletService.rejectInvite(employeeId, membershipId))
			.isInstanceOf(BadRequestException.class)
			.hasMessage("Only pending invitations can be rejected");
	}

	@Test
	void getMyInvitesReturnsOnlyPendingInvitationsInPageResponse() {
		UUID userId = UUID.randomUUID();
		User employee = user(userId, "employee@example.com", "Employee");
		User owner = user(UUID.randomUUID(), "owner@example.com", "Owner");
		Outlet outlet = outlet(UUID.randomUUID(), "Outlet A");
		OutletMembership invite = membership(UUID.randomUUID(), outlet, employee, OutletRole.EMPLOYEE, OutletMembershipStatus.INVITED);
		invite.setInvitedBy(owner);
		when(outletMembershipRepository.findAllByUser_IdAndStatusAndRemovedAtIsNull(
			userId,
			OutletMembershipStatus.INVITED,
			PageRequest.of(0, 20, org.springframework.data.domain.Sort.by(org.springframework.data.domain.Sort.Direction.DESC, "updatedAt"))
		)).thenReturn(new PageImpl<>(List.of(invite), PageRequest.of(0, 20), 1));

		PageResponse<OutletMembershipResponse> invites = outletService.getMyInvites(userId, PageRequest.of(0, 20));

		assertThat(invites.content()).hasSize(1);
		assertThat(invites.content().getFirst().status()).isEqualTo(OutletMembershipStatus.INVITED);
		assertThat(invites.content().getFirst().invitedByUserName()).isEqualTo("Owner");
		assertThat(invites.totalElements()).isEqualTo(1);
		assertThat(invites.first()).isTrue();
	}

	@Test
	void removeMembershipSoftDeletesEmployeeMembership() {
		UUID outletId = UUID.randomUUID();
		UUID ownerId = UUID.randomUUID();
		UUID employeeId = UUID.randomUUID();
		UUID membershipId = UUID.randomUUID();
		User owner = user(ownerId, "owner@example.com", "Owner");
		User employee = user(employeeId, "employee@example.com", "Employee");
		Outlet outlet = outlet(outletId, "Outlet A");
		OutletMembership ownerMembership = membership(UUID.randomUUID(), outlet, owner, OutletRole.OWNER, OutletMembershipStatus.ACCEPTED);
		OutletMembership employeeMembership = membership(membershipId, outlet, employee, OutletRole.EMPLOYEE, OutletMembershipStatus.ACCEPTED);

		when(outletMembershipRepository.findByOutlet_IdAndUser_IdAndRemovedAtIsNull(outletId, ownerId))
			.thenReturn(Optional.of(ownerMembership));
		when(outletMembershipRepository.findDetailedByIdAndRemovedAtIsNull(membershipId))
			.thenReturn(Optional.of(employeeMembership));

		outletService.removeMembership(ownerId, outletId, membershipId);

		assertThat(employeeMembership.getRemovedAt()).isNotNull();
		assertThat(employeeMembership.getRemovedBy()).isEqualTo(owner);
		verify(outletMembershipRepository).save(employeeMembership);
	}

	@Test
	void removeMembershipRejectsOwnerMembershipRemoval() {
		UUID outletId = UUID.randomUUID();
		UUID ownerId = UUID.randomUUID();
		UUID membershipId = UUID.randomUUID();
		User owner = user(ownerId, "owner@example.com", "Owner");
		Outlet outlet = outlet(outletId, "Outlet A");
		OutletMembership ownerMembership = membership(UUID.randomUUID(), outlet, owner, OutletRole.OWNER, OutletMembershipStatus.ACCEPTED);
		OutletMembership targetOwnerMembership = membership(membershipId, outlet, owner, OutletRole.OWNER, OutletMembershipStatus.ACCEPTED);

		when(outletMembershipRepository.findByOutlet_IdAndUser_IdAndRemovedAtIsNull(outletId, ownerId))
			.thenReturn(Optional.of(ownerMembership));
		when(outletMembershipRepository.findDetailedByIdAndRemovedAtIsNull(membershipId))
			.thenReturn(Optional.of(targetOwnerMembership));

		assertThatThrownBy(() -> outletService.removeMembership(ownerId, outletId, membershipId))
			.isInstanceOf(BadRequestException.class)
			.hasMessage("Owner memberships cannot be removed through this endpoint");
	}

	private User user(UUID id, String email, String name) {
		User user = new User();
		user.setId(id);
		user.setEmail(email);
		user.setName(name);
		return user;
	}

	private Outlet outlet(UUID id, String name) {
		Outlet outlet = new Outlet();
		outlet.setId(id);
		outlet.setName(name);
		outlet.setLatitude(new BigDecimal("12.9715987"));
		outlet.setLongitude(new BigDecimal("77.5945627"));
		outlet.setRadiusMeters(150);
		return outlet;
	}

	private OutletMembership membership(
		UUID id,
		Outlet outlet,
		User user,
		OutletRole role,
		OutletMembershipStatus status
	) {
		OutletMembership membership = new OutletMembership();
		membership.setId(id);
		membership.setOutlet(outlet);
		membership.setUser(user);
		membership.setRole(role);
		membership.setStatus(status);
		return membership;
	}

}
