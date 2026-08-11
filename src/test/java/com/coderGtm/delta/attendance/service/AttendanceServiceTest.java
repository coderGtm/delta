package com.coderGtm.delta.attendance.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import java.math.BigDecimal;
import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.List;
import java.util.Optional;
import java.util.UUID;

import org.junit.jupiter.api.BeforeEach;
import org.springframework.data.domain.PageImpl;
import org.springframework.data.domain.PageRequest;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import com.coderGtm.delta.attendance.dto.AttendanceEntryResponse;
import com.coderGtm.delta.attendance.dto.CreateAttendanceEntryRequest;
import com.coderGtm.delta.attendance.dto.ManageAttendanceEntryRequest;
import com.coderGtm.delta.attendance.dto.UpdateAttendanceEntryRequest;
import com.coderGtm.delta.attendance.entity.AttendanceEntry;
import com.coderGtm.delta.attendance.entity.AttendanceEntryType;
import com.coderGtm.delta.attendance.mapper.AttendanceMapper;
import com.coderGtm.delta.attendance.repository.AttendanceEntryRepository;
import com.coderGtm.delta.common.audit.service.AuditService;
import com.coderGtm.delta.common.dto.PageResponse;
import com.coderGtm.delta.common.exception.BadRequestException;
import com.coderGtm.delta.common.metrics.ApplicationMetrics;
import com.coderGtm.delta.common.exception.ForbiddenException;
import com.coderGtm.delta.outlet.entity.Outlet;
import com.coderGtm.delta.outlet.entity.OutletMembership;
import com.coderGtm.delta.outlet.entity.OutletMembershipStatus;
import com.coderGtm.delta.outlet.entity.OutletRole;
import com.coderGtm.delta.outlet.repository.OutletMembershipRepository;
import com.coderGtm.delta.user.User;

@ExtendWith(MockitoExtension.class)
class AttendanceServiceTest {

	private static final Instant FIXED_NOW = Instant.parse("2024-05-01T09:15:30Z");

	@Mock
	private AttendanceEntryRepository attendanceEntryRepository;

	@Mock
	private OutletMembershipRepository outletMembershipRepository;

	@Mock
	private AuditService auditService;

	@Mock
	private ApplicationMetrics applicationMetrics;

	private AttendanceService attendanceService;

	@BeforeEach
	void setUp() {
		attendanceService = new AttendanceService(
			attendanceEntryRepository,
			outletMembershipRepository,
			new AttendanceMapper(),
			Clock.fixed(FIXED_NOW, ZoneOffset.UTC),
			auditService,
			applicationMetrics
		);
	}

	@Test
	void createOwnEntryUsesServerClockForAcceptedEmployee() {
		UUID outletId = UUID.randomUUID();
		UUID employeeId = UUID.randomUUID();
		User employee = user(employeeId, "employee@example.com", "Employee");
		Outlet outlet = outlet(outletId, "Outlet A");
		OutletMembership membership = membership(
			UUID.randomUUID(),
			outlet,
			employee,
			OutletRole.EMPLOYEE,
			OutletMembershipStatus.ACCEPTED
		);

		when(outletMembershipRepository.findByOutlet_IdAndUser_IdAndRemovedAtIsNull(outletId, employeeId))
			.thenReturn(Optional.of(membership));
		when(attendanceEntryRepository.save(any(AttendanceEntry.class))).thenAnswer(invocation -> {
			AttendanceEntry saved = invocation.getArgument(0);
			saved.setId(UUID.randomUUID());
			return saved;
		});

		AttendanceEntryResponse response = attendanceService.createOwnEntry(
			employeeId,
			outletId,
			new CreateAttendanceEntryRequest(
				AttendanceEntryType.CLOCK_IN,
				new BigDecimal("12.9715987"),
				new BigDecimal("77.5945627")
			)
		);

		ArgumentCaptor<AttendanceEntry> captor = ArgumentCaptor.forClass(AttendanceEntry.class);
		verify(attendanceEntryRepository).save(captor.capture());

		assertThat(captor.getValue().getEntryTime()).isEqualTo(FIXED_NOW);
		assertThat(response.entryTime()).isEqualTo(FIXED_NOW);
		assertThat(response.userId()).isEqualTo(employeeId);
		assertThat(response.displayName()).isEqualTo("Employee");
		assertThat(response.type()).isEqualTo(AttendanceEntryType.CLOCK_IN);
	}

	@Test
	void createOwnEntryRejectsAcceptedOwner() {
		UUID outletId = UUID.randomUUID();
		UUID ownerId = UUID.randomUUID();
		User owner = user(ownerId, "owner@example.com", "Owner");
		Outlet outlet = outlet(outletId, "Outlet A");
		OutletMembership ownerMembership = membership(
			UUID.randomUUID(),
			outlet,
			owner,
			OutletRole.OWNER,
			OutletMembershipStatus.ACCEPTED
		);

		when(outletMembershipRepository.findByOutlet_IdAndUser_IdAndRemovedAtIsNull(outletId, ownerId))
			.thenReturn(Optional.of(ownerMembership));

		assertThatThrownBy(() -> attendanceService.createOwnEntry(
			ownerId,
			outletId,
			new CreateAttendanceEntryRequest(
				AttendanceEntryType.CLOCK_IN,
				new BigDecimal("12.9715987"),
				new BigDecimal("77.5945627")
			)
		))
			.isInstanceOf(ForbiddenException.class)
			.hasMessage("Only accepted employees can create their own attendance entries");
	}

	@Test
	void createOwnEntryRejectsOutsideGeofenceWhenEnabled() {
		UUID outletId = UUID.randomUUID();
		UUID employeeId = UUID.randomUUID();
		User employee = user(employeeId, "employee@example.com", "Employee");
		Outlet outlet = outlet(outletId, "Outlet A");
		outlet.setGeofenceEnabled(true);
		OutletMembership membership = membership(
			UUID.randomUUID(),
			outlet,
			employee,
			OutletRole.EMPLOYEE,
			OutletMembershipStatus.ACCEPTED
		);

		when(outletMembershipRepository.findByOutlet_IdAndUser_IdAndRemovedAtIsNull(outletId, employeeId))
			.thenReturn(Optional.of(membership));

		assertThatThrownBy(() -> attendanceService.createOwnEntry(
			employeeId,
			outletId,
			new CreateAttendanceEntryRequest(
				AttendanceEntryType.CLOCK_IN,
				new BigDecimal("13.0352000"),
				new BigDecimal("77.5970000")
			)
		))
			.isInstanceOf(ForbiddenException.class)
			.hasMessage("Attendance location is outside the outlet geofence");
	}

	@Test
	void createManagedEntryRequiresAcceptedEmployeeMembership() {
		UUID outletId = UUID.randomUUID();
		UUID ownerId = UUID.randomUUID();
		UUID employeeId = UUID.randomUUID();

		User owner = user(ownerId, "owner@example.com", "Owner");
		User employee = user(employeeId, "employee@example.com", "Employee");
		Outlet outlet = outlet(outletId, "Outlet A");

		OutletMembership ownerMembership = membership(
			UUID.randomUUID(),
			outlet,
			owner,
			OutletRole.OWNER,
			OutletMembershipStatus.ACCEPTED
		);
		OutletMembership invitedEmployeeMembership = membership(
			UUID.randomUUID(),
			outlet,
			employee,
			OutletRole.EMPLOYEE,
			OutletMembershipStatus.INVITED
		);

		when(outletMembershipRepository.findByOutlet_IdAndUser_IdAndRemovedAtIsNull(outletId, ownerId))
			.thenReturn(Optional.of(ownerMembership));
		when(outletMembershipRepository.findByOutlet_IdAndUser_IdAndRemovedAtIsNull(outletId, employeeId))
			.thenReturn(Optional.of(invitedEmployeeMembership));

		assertThatThrownBy(() -> attendanceService.createManagedEntry(
			ownerId,
			outletId,
			new ManageAttendanceEntryRequest(
				employeeId,
				AttendanceEntryType.CLOCK_IN,
				Instant.parse("2024-04-30T12:00:00Z"),
				new BigDecimal("12.9715987"),
				new BigDecimal("77.5945627")
			)
		))
			.isInstanceOf(BadRequestException.class)
			.hasMessage("Attendance can only be created for accepted employee memberships");
	}

	@Test
	void getAttendanceEntriesForOwnerKeepsMembershipDisplayNameForRemovedEmployees() {
		UUID outletId = UUID.randomUUID();
		UUID ownerId = UUID.randomUUID();
		UUID removedEmployeeId = UUID.randomUUID();

		User owner = user(ownerId, "owner@example.com", "Owner");
		User removedEmployee = user(removedEmployeeId, "employee@example.com", "Employee");
		Outlet outlet = outlet(outletId, "Outlet A");

		OutletMembership ownerMembership = membership(
			UUID.randomUUID(),
			outlet,
			owner,
			OutletRole.OWNER,
			OutletMembershipStatus.ACCEPTED
		);
		OutletMembership removedEmployeeMembership = membership(
			UUID.randomUUID(),
			outlet,
			removedEmployee,
			OutletRole.EMPLOYEE,
			OutletMembershipStatus.ACCEPTED
		);
		removedEmployeeMembership.setDisplayName("Nickname");
		removedEmployeeMembership.setRemovedAt(Instant.parse("2024-04-21T00:00:00Z"));
		AttendanceEntry historicalEntry = attendanceEntry(
			UUID.randomUUID(),
			outlet,
			removedEmployee,
			AttendanceEntryType.CLOCK_OUT,
			Instant.parse("2024-04-20T18:00:00Z")
		);

		when(outletMembershipRepository.findByOutlet_IdAndUser_IdAndRemovedAtIsNull(outletId, ownerId))
			.thenReturn(Optional.of(ownerMembership));
		when(outletMembershipRepository.findAllByOutlet_Id(outletId))
			.thenReturn(List.of(ownerMembership, removedEmployeeMembership));
		when(attendanceEntryRepository.findAllByOutlet_IdAndUser_Id(
			outletId,
			removedEmployeeId,
			PageRequest.of(
				0,
				20,
				org.springframework.data.domain.Sort.by(
					org.springframework.data.domain.Sort.Order.desc("entryTime"),
					org.springframework.data.domain.Sort.Order.desc("createdAt")
				)
			)
		)).thenReturn(new PageImpl<>(List.of(historicalEntry), PageRequest.of(0, 20), 1));

		PageResponse<AttendanceEntryResponse> responses = attendanceService.getAttendanceEntries(
			ownerId,
			outletId,
			removedEmployeeId,
			PageRequest.of(0, 20)
		);

		assertThat(responses.content()).hasSize(1);
		assertThat(responses.content().getFirst().userId()).isEqualTo(removedEmployeeId);
		assertThat(responses.content().getFirst().displayName()).isEqualTo("Nickname");
		assertThat(responses.content().getFirst().userName()).isEqualTo("Employee");
		assertThat(responses.totalElements()).isEqualTo(1);
		verify(outletMembershipRepository, never())
			.findByOutlet_IdAndUser_IdAndRemovedAtIsNull(outletId, removedEmployeeId);
	}

	@Test
	void getAttendanceEntryRejectsEmployeeAccessToAnotherUsersEntry() {
		UUID outletId = UUID.randomUUID();
		UUID employeeId = UUID.randomUUID();
		UUID otherEmployeeId = UUID.randomUUID();
		UUID attendanceEntryId = UUID.randomUUID();

		User employee = user(employeeId, "employee@example.com", "Employee");
		User otherEmployee = user(otherEmployeeId, "other@example.com", "Other");
		Outlet outlet = outlet(outletId, "Outlet A");

		OutletMembership employeeMembership = membership(
			UUID.randomUUID(),
			outlet,
			employee,
			OutletRole.EMPLOYEE,
			OutletMembershipStatus.ACCEPTED
		);
		AttendanceEntry otherEntry = attendanceEntry(
			attendanceEntryId,
			outlet,
			otherEmployee,
			AttendanceEntryType.CLOCK_IN,
			Instant.parse("2024-04-20T09:00:00Z")
		);

		when(outletMembershipRepository.findByOutlet_IdAndUser_IdAndRemovedAtIsNull(outletId, employeeId))
			.thenReturn(Optional.of(employeeMembership));
		when(attendanceEntryRepository.findDetailedByIdAndOutlet_Id(attendanceEntryId, outletId))
			.thenReturn(Optional.of(otherEntry));

		assertThatThrownBy(() -> attendanceService.getAttendanceEntry(employeeId, outletId, attendanceEntryId))
			.isInstanceOf(ForbiddenException.class)
			.hasMessage("Employees can only view their own attendance entries");
	}

	@Test
	void updateAttendanceEntryAllowsOwnerToEditHistoricalEntryAfterMembershipRemoval() {
		UUID outletId = UUID.randomUUID();
		UUID ownerId = UUID.randomUUID();
		UUID attendanceEntryId = UUID.randomUUID();

		User owner = user(ownerId, "owner@example.com", "Owner");
		User removedEmployee = user(UUID.randomUUID(), "employee@example.com", "Employee");
		Outlet outlet = outlet(outletId, "Outlet A");

		OutletMembership ownerMembership = membership(
			UUID.randomUUID(),
			outlet,
			owner,
			OutletRole.OWNER,
			OutletMembershipStatus.ACCEPTED
		);
		OutletMembership removedEmployeeMembership = membership(
			UUID.randomUUID(),
			outlet,
			removedEmployee,
			OutletRole.EMPLOYEE,
			OutletMembershipStatus.ACCEPTED
		);
		removedEmployeeMembership.setDisplayName("Nickname");
		removedEmployeeMembership.setRemovedAt(Instant.parse("2024-04-21T00:00:00Z"));
		AttendanceEntry historicalEntry = attendanceEntry(
			attendanceEntryId,
			outlet,
			removedEmployee,
			AttendanceEntryType.CLOCK_IN,
			Instant.parse("2024-04-20T09:00:00Z")
		);

		when(outletMembershipRepository.findByOutlet_IdAndUser_IdAndRemovedAtIsNull(outletId, ownerId))
			.thenReturn(Optional.of(ownerMembership));
		when(outletMembershipRepository.findByOutlet_IdAndUser_Id(outletId, removedEmployee.getId()))
			.thenReturn(Optional.of(removedEmployeeMembership));
		when(attendanceEntryRepository.findDetailedByIdAndOutlet_Id(attendanceEntryId, outletId))
			.thenReturn(Optional.of(historicalEntry));
		when(attendanceEntryRepository.save(historicalEntry)).thenReturn(historicalEntry);

		AttendanceEntryResponse response = attendanceService.updateAttendanceEntry(
			ownerId,
			outletId,
			attendanceEntryId,
			new UpdateAttendanceEntryRequest(
				AttendanceEntryType.CLOCK_OUT,
				Instant.parse("2024-04-20T18:10:00Z"),
				new BigDecimal("13.0000000"),
				new BigDecimal("78.0000000")
			)
		);

		assertThat(response.type()).isEqualTo(AttendanceEntryType.CLOCK_OUT);
		assertThat(response.entryTime()).isEqualTo(Instant.parse("2024-04-20T18:10:00Z"));
		assertThat(response.latitude()).isEqualByComparingTo("13.0000000");
		assertThat(response.displayName()).isEqualTo("Nickname");
	}

	@Test
	void deleteAttendanceEntryAllowsOwnerToDeleteHistoricalEntryAfterMembershipRemoval() {
		UUID outletId = UUID.randomUUID();
		UUID ownerId = UUID.randomUUID();
		UUID attendanceEntryId = UUID.randomUUID();

		User owner = user(ownerId, "owner@example.com", "Owner");
		User removedEmployee = user(UUID.randomUUID(), "employee@example.com", "Employee");
		Outlet outlet = outlet(outletId, "Outlet A");

		OutletMembership ownerMembership = membership(
			UUID.randomUUID(),
			outlet,
			owner,
			OutletRole.OWNER,
			OutletMembershipStatus.ACCEPTED
		);
		AttendanceEntry historicalEntry = attendanceEntry(
			attendanceEntryId,
			outlet,
			removedEmployee,
			AttendanceEntryType.CLOCK_IN,
			Instant.parse("2024-04-20T09:00:00Z")
		);

		when(outletMembershipRepository.findByOutlet_IdAndUser_IdAndRemovedAtIsNull(outletId, ownerId))
			.thenReturn(Optional.of(ownerMembership));
		when(attendanceEntryRepository.findDetailedByIdAndOutlet_Id(attendanceEntryId, outletId))
			.thenReturn(Optional.of(historicalEntry));

		attendanceService.deleteAttendanceEntry(ownerId, outletId, attendanceEntryId);

		verify(attendanceEntryRepository).delete(historicalEntry);
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
		membership.setDisplayName(user.getName());
		membership.setRole(role);
		membership.setStatus(status);
		return membership;
	}

	private AttendanceEntry attendanceEntry(
		UUID id,
		Outlet outlet,
		User user,
		AttendanceEntryType type,
		Instant entryTime
	) {
		AttendanceEntry entry = new AttendanceEntry();
		entry.setId(id);
		entry.setOutlet(outlet);
		entry.setUser(user);
		entry.setType(type);
		entry.setEntryTime(entryTime);
		entry.setLatitude(new BigDecimal("12.9715987"));
		entry.setLongitude(new BigDecimal("77.5945627"));
		return entry;
	}
}
